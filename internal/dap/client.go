package dap

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/docker/buildx/builder"
	"github.com/docker/buildx/commands"
	buildxdap "github.com/docker/buildx/dap"
	"github.com/docker/buildx/dap/common"
	_ "github.com/docker/buildx/driver/docker"
	_ "github.com/docker/buildx/driver/docker-container"
	_ "github.com/docker/buildx/driver/kubernetes"
	"github.com/docker/buildx/store/storeutil"
	"github.com/docker/buildx/util/progress"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/docker/client"
	godap "github.com/google/go-dap"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/tsukinoko-kun/harbor/internal/terminal"
)

// DebugParams contains the parameters for starting a debug session.
// This struct is extensible - add new fields as needed.
type DebugParams struct {
	// Dockerfile is the path to the Dockerfile to debug.
	Dockerfile string
	// Context is the build context directory for the Dockerfile.
	Context string
}

// LaunchConfig matches the buildx DAP launch configuration format.
type LaunchConfig struct {
	Dockerfile  string `json:"dockerfile,omitempty"`
	ContextPath string `json:"contextPath,omitempty"`
	Target      string `json:"target,omitempty"`
	common.Config
}

// GetConfig implements the buildxdap.LaunchConfig interface.
func (c LaunchConfig) GetConfig() common.Config {
	return c.Config
}

// pipeConn implements buildxdap.Conn for a net.Conn pipe.
type pipeConn struct {
	conn   net.Conn
	reader *bufio.Reader
	mu     sync.Mutex
}

func newPipeConn(conn net.Conn) *pipeConn {
	return &pipeConn{
		conn:   conn,
		reader: bufio.NewReader(conn),
	}
}

func (c *pipeConn) SendMsg(m godap.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return godap.WriteProtocolMessage(c.conn, m)
}

func (c *pipeConn) RecvMsg(ctx context.Context) (godap.Message, error) {
	// Create a channel to receive the result
	type result struct {
		msg godap.Message
		err error
	}
	ch := make(chan result, 1)

	go func() {
		msg, err := godap.ReadProtocolMessage(c.reader)
		ch <- result{msg, err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		return r.msg, r.err
	}
}

func (c *pipeConn) Close() error {
	return c.conn.Close()
}

// ConsoleTerminal is the terminal emulator for the debug console.
// It processes ANSI escape sequences for color and cursor movement.
type ConsoleTerminal = terminal.Terminal

// DapClient is the debug adapter protocol client.
// It manages the connection to a DAP server and provides
// a channel for notifying the UI of updates.
type DapClient struct {
	// Params contains the debug session parameters.
	Params DebugParams
	// UpdateChan is used to signal the UI that data has changed.
	// Send to this channel when the DAP server sends new data.
	UpdateChan chan struct{}
	// Stopped indicates whether the debugger is currently paused.
	Stopped bool
	// StoppedThreadId is the thread ID from the last StoppedEvent,
	// needed for Next/Continue requests.
	StoppedThreadId int
	// CurrentLine is the line number from the current stack frame (1-indexed).
	// This is updated when the debugger stops.
	CurrentLine int
	// CurrentEndLine is the end line of the current execution range (1-indexed).
	// This will be 0 if the DAP server doesn't provide range information.
	CurrentEndLine int
	// CurrentColumn is the column number from the current stack frame (1-indexed).
	CurrentColumn int
	// CurrentEndColumn is the end column of the current execution range (1-indexed).
	CurrentEndColumn int
	// Console is the terminal emulator for shell output.
	// It processes ANSI escape sequences for color and cursor movement.
	Console *terminal.Terminal
	// EvaluatePending indicates whether an evaluate request is in-flight.
	EvaluatePending bool
	// pendingExpression stores the expression being evaluated.
	pendingExpression string

	ctx          context.Context
	cancel       context.CancelFunc
	clientConn   net.Conn
	clientReader *bufio.Reader
	adapter      *buildxdap.Adapter[LaunchConfig]
	seq          int64

	// Shell session state
	shellConn  net.Conn   // Persistent connection to the shell
	shellMu    sync.Mutex // Protects shellConn
	shellReady bool       // True when shell is ready for commands
}

// Client is the global DAP client instance.
// It starts as nil and is set when a debug session is started.
var Client *DapClient

// NewClient creates a new DAP client with the given parameters.
// It starts an embedded Buildx DAP server, initializes the protocol,
// and launches the build.
func NewClient(params DebugParams) (*DapClient, error) {
	log.Printf("Starting debugger with parameters:")
	log.Printf("  Dockerfile: %s", params.Dockerfile)
	log.Printf("  Context: %s", params.Context)

	ctx, cancel := context.WithCancel(context.Background())

	// Create in-memory pipe for communication
	// clientConn: we write DAP requests here and read responses
	// serverConn: the adapter reads requests here and writes responses
	clientConn, serverConn := net.Pipe()

	// Create the buildx DAP adapter
	adapter := buildxdap.New[LaunchConfig]()

	// Create our custom conn wrapper for the server side
	serverDapConn := newPipeConn(serverConn)

	// Channel to signal when adapter is ready to receive
	adapterStarted := make(chan struct{})

	// Start the adapter in a goroutine - it will wait for DAP messages
	go func() {
		close(adapterStarted) // Signal that we're about to start
		_, err := adapter.Start(ctx, serverDapConn)
		if err != nil {
			log.Printf("DAP Adapter exited: %v", err)
		}
	}()

	// Wait for adapter goroutine to start
	<-adapterStarted

	c := &DapClient{
		Params:       params,
		UpdateChan:   make(chan struct{}),
		Console:      terminal.New(),
		ctx:          ctx,
		cancel:       cancel,
		clientConn:   clientConn,
		clientReader: bufio.NewReader(clientConn),
		adapter:      adapter,
		seq:          0,
	}

	// Send Initialize request
	if err := c.sendInitialize(); err != nil {
		c.Close()
		return nil, err
	}

	// Send Launch request
	if err := c.sendLaunch(); err != nil {
		c.Close()
		return nil, err
	}

	// Send ConfigurationDone to start the build
	if err := c.sendConfigurationDone(); err != nil {
		c.Close()
		return nil, err
	}

	// Start listening for events from the DAP server
	go c.eventLoop()

	// Start the build through buildx with the DAP handler
	go c.runBuildWithHandler()

	return c, nil
}

// Close closes the DAP client connection and cleans up resources.
// This method is idempotent and will never panic or return errors.
func (c *DapClient) Close() {
	if c == nil {
		return
	}
	defer func() { recover() }()

	if c.adapter != nil {
		_ = c.adapter.Stop()
	}
	if c.cancel != nil {
		c.cancel()
	}
	if c.clientConn != nil {
		_ = c.clientConn.Close()
	}
	// Note: We don't close UpdateChan here because background goroutines
	// may still try to send on it. The channel will be garbage collected
	// when the client is no longer referenced.
}

// nextSeq returns the next sequence number for DAP messages.
func (c *DapClient) nextSeq() int {
	return int(atomic.AddInt64(&c.seq, 1))
}

// sendRequest sends a DAP request over the connection.
func (c *DapClient) sendRequest(request godap.Message) error {
	return godap.WriteProtocolMessage(c.clientConn, request)
}

// readResponse reads a DAP response from the connection.
func (c *DapClient) readResponse() (godap.Message, error) {
	return godap.ReadProtocolMessage(c.clientReader)
}

// sendInitialize sends the DAP initialize request.
func (c *DapClient) sendInitialize() error {
	req := &godap.InitializeRequest{
		Request: godap.Request{
			ProtocolMessage: godap.ProtocolMessage{
				Seq:  c.nextSeq(),
				Type: "request",
			},
			Command: "initialize",
		},
		Arguments: godap.InitializeRequestArguments{
			ClientID:                     "harbor",
			ClientName:                   "Harbor",
			LinesStartAt1:                true,
			ColumnsStartAt1:              true,
			SupportsRunInTerminalRequest: true,
		},
	}

	if err := c.sendRequest(req); err != nil {
		return err
	}

	// Read responses until we get the InitializeResponse
	for {
		msg, err := c.readResponse()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if _, ok := msg.(*godap.InitializeResponse); ok {
			return nil
		}
		// Continue reading (might get events first)
	}
}

// sendLaunch sends the DAP launch request with build configuration.
func (c *DapClient) sendLaunch() error {
	args := LaunchConfig{
		Dockerfile:  c.Params.Dockerfile,
		ContextPath: c.Params.Context,
		Config: common.Config{
			StopOnEntry: true,
		},
	}

	rawArgs, err := json.Marshal(args)
	if err != nil {
		return err
	}

	req := &godap.LaunchRequest{
		Request: godap.Request{
			ProtocolMessage: godap.ProtocolMessage{
				Seq:  c.nextSeq(),
				Type: "request",
			},
			Command: "launch",
		},
		Arguments: rawArgs,
	}

	if err := c.sendRequest(req); err != nil {
		return err
	}

	// Read responses until we get the LaunchResponse
	for {
		msg, err := c.readResponse()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if _, ok := msg.(*godap.LaunchResponse); ok {
			return nil
		}
		// Continue reading (might get events first)
	}
}

// sendConfigurationDone sends the DAP configurationDone request to start the build.
func (c *DapClient) sendConfigurationDone() error {
	req := &godap.ConfigurationDoneRequest{
		Request: godap.Request{
			ProtocolMessage: godap.ProtocolMessage{
				Seq:  c.nextSeq(),
				Type: "request",
			},
			Command: "configurationDone",
		},
	}

	if err := c.sendRequest(req); err != nil {
		return err
	}

	// Read responses until we get the ConfigurationDoneResponse
	for {
		msg, err := c.readResponse()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if _, ok := msg.(*godap.ConfigurationDoneResponse); ok {
			return nil
		}
		// Continue reading (might get events first)
	}
}

// eventLoop listens for events from the DAP server and logs them.
func (c *DapClient) eventLoop() {
	log.Println("[DAP] Event loop started")
	for {
		select {
		case <-c.ctx.Done():
			log.Println("[DAP] Event loop stopped (context cancelled)")
			return
		default:
		}

		msg, err := c.readResponse()
		if err != nil {
			if err == io.EOF || c.ctx.Err() != nil {
				log.Println("[DAP] Event loop ended (connection closed)")
				return
			}
			log.Printf("[DAP] Error reading message: %v", err)
			return
		}

		c.handleMessage(msg)
	}
}

// handleMessage processes a DAP message and logs it appropriately.
func (c *DapClient) handleMessage(msg godap.Message) {
	switch m := msg.(type) {
	case *godap.TerminatedEvent:
		log.Println("[DAP] Build terminated")
		// Notify UI that data changed
		select {
		case c.UpdateChan <- struct{}{}:
		default:
		}

	case *godap.ExitedEvent:
		log.Printf("[DAP] Build exited with code: %d", m.Body.ExitCode)
		// Notify UI that data changed
		select {
		case c.UpdateChan <- struct{}{}:
		default:
		}

	case *godap.OutputEvent:
		// Log output from the build process
		category := m.Body.Category
		if category == "" {
			category = "console"
		}
		log.Printf("[DAP] Output [%s]: %s", category, m.Body.Output)

	case *godap.StoppedEvent:
		log.Printf("[DAP] Stopped: reason=%s, threadId=%d", m.Body.Reason, m.Body.ThreadId)
		c.Stopped = true
		c.StoppedThreadId = m.Body.ThreadId
		// Fetch the current stack trace to get line number
		if err := c.GetStackTrace(); err != nil {
			log.Printf("[DAP] Failed to get stack trace: %v", err)
		}
		// Notify UI that data changed
		select {
		case c.UpdateChan <- struct{}{}:
		default:
		}

	case *godap.ThreadEvent:
		log.Printf("[DAP] Thread event: reason=%s, threadId=%d", m.Body.Reason, m.Body.ThreadId)

	case *godap.ContinuedEvent:
		log.Printf("[DAP] Continued: threadId=%d", m.Body.ThreadId)
		c.Stopped = false
		// Notify UI that data changed
		select {
		case c.UpdateChan <- struct{}{}:
		default:
		}

	case *godap.BreakpointEvent:
		log.Printf("[DAP] Breakpoint event: reason=%s", m.Body.Reason)

	case *godap.InitializedEvent:
		log.Println("[DAP] Initialized event received")

	case *godap.EvaluateResponse:
		log.Printf("[DAP] Evaluate response: success=%v, result=%q", m.Success, m.Body.Result)
		c.EvaluatePending = false

	case *godap.ErrorResponse:
		log.Printf("[DAP] Error response: command=%s, message=%s", m.Command, m.Message)
		// Check if this is a response to an evaluate request
		if m.Command == "evaluate" && c.EvaluatePending {
			c.EvaluatePending = false
			// Append error message to console output (red color)
			c.appendToConsole(fmt.Sprintf("\n\x1b[31m[Error] %s\x1b[0m\n", m.Message))
		}

	case *godap.RunInTerminalRequest:
		// Server is asking us to run a command in a terminal
		log.Printf("[DAP] RunInTerminal request: args=%v, cwd=%s", m.Arguments.Args, m.Arguments.Cwd)
		c.handleRunInTerminal(m)

	default:
		// Log unknown messages for debugging
		log.Printf("[DAP] Unhandled message: %T %+v", msg, msg)
	}
}

// SendNext sends a DAP next (step over) request.
func (c *DapClient) SendNext() error {
	if !c.Stopped {
		return nil
	}

	req := &godap.NextRequest{
		Request: godap.Request{
			ProtocolMessage: godap.ProtocolMessage{
				Seq:  c.nextSeq(),
				Type: "request",
			},
			Command: "next",
		},
		Arguments: godap.NextArguments{
			ThreadId: c.StoppedThreadId,
		},
	}

	c.Stopped = false
	return c.sendRequest(req)
}

// SendContinue sends a DAP continue request.
func (c *DapClient) SendContinue() error {
	if !c.Stopped {
		return nil
	}

	req := &godap.ContinueRequest{
		Request: godap.Request{
			ProtocolMessage: godap.ProtocolMessage{
				Seq:  c.nextSeq(),
				Type: "request",
			},
			Command: "continue",
		},
		Arguments: godap.ContinueArguments{
			ThreadId: c.StoppedThreadId,
		},
	}

	c.Stopped = false
	return c.sendRequest(req)
}

// SendStepIn sends a DAP stepIn request.
func (c *DapClient) SendStepIn() error {
	if !c.Stopped {
		return nil
	}

	req := &godap.StepInRequest{
		Request: godap.Request{
			ProtocolMessage: godap.ProtocolMessage{
				Seq:  c.nextSeq(),
				Type: "request",
			},
			Command: "stepIn",
		},
		Arguments: godap.StepInArguments{
			ThreadId: c.StoppedThreadId,
		},
	}

	c.Stopped = false
	return c.sendRequest(req)
}

// SendStepOut sends a DAP stepOut request.
func (c *DapClient) SendStepOut() error {
	if !c.Stopped {
		return nil
	}

	req := &godap.StepOutRequest{
		Request: godap.Request{
			ProtocolMessage: godap.ProtocolMessage{
				Seq:  c.nextSeq(),
				Type: "request",
			},
			Command: "stepOut",
		},
		Arguments: godap.StepOutArguments{
			ThreadId: c.StoppedThreadId,
		},
	}

	c.Stopped = false
	return c.sendRequest(req)
}

// GetStackTrace sends a DAP stackTrace request and updates CurrentLine.
// This should be called when the debugger is stopped to get the current execution position.
func (c *DapClient) GetStackTrace() error {
	if !c.Stopped {
		return nil
	}

	req := &godap.StackTraceRequest{
		Request: godap.Request{
			ProtocolMessage: godap.ProtocolMessage{
				Seq:  c.nextSeq(),
				Type: "request",
			},
			Command: "stackTrace",
		},
		Arguments: godap.StackTraceArguments{
			ThreadId: c.StoppedThreadId,
		},
	}

	if err := c.sendRequest(req); err != nil {
		return err
	}

	// Read responses until we get the StackTraceResponse
	for {
		msg, err := c.readResponse()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if resp, ok := msg.(*godap.StackTraceResponse); ok {
			// Extract line and range information from the top stack frame
			if len(resp.Body.StackFrames) > 0 {
				frame := resp.Body.StackFrames[0]
				c.CurrentLine = frame.Line
				c.CurrentEndLine = frame.EndLine // Will be 0 if not provided
				c.CurrentColumn = frame.Column
				c.CurrentEndColumn = frame.EndColumn // Will be 0 if not provided
				if c.CurrentEndLine > 0 {
					log.Printf("[DAP] Current range: lines %d-%d", c.CurrentLine, c.CurrentEndLine)
				} else {
					log.Printf("[DAP] Current line: %d", c.CurrentLine)
				}
			}
			return nil
		}
		// Handle other messages that might come in while waiting
		c.handleMessage(msg)
	}
}

// SendEvaluate sends a shell command to execute in the build container.
func (c *DapClient) SendEvaluate(expression string) error {
	c.shellMu.Lock()
	hasShell := c.shellConn != nil && c.shellReady
	c.shellMu.Unlock()

	// If we have an active shell, send command directly
	if hasShell {
		return c.sendShellCommand(expression)
	}

	// Otherwise, open a shell session first
	if c.EvaluatePending {
		return fmt.Errorf("shell session is starting, please wait")
	}

	c.EvaluatePending = true
	c.pendingExpression = expression

	// Send "exec" to open the shell
	seq := c.nextSeq()
	req := &godap.EvaluateRequest{
		Request: godap.Request{
			ProtocolMessage: godap.ProtocolMessage{
				Seq:  seq,
				Type: "request",
			},
			Command: "evaluate",
		},
		Arguments: godap.EvaluateArguments{
			Expression: "exec",
			Context:    "repl",
		},
	}

	log.Printf("[DAP] Opening shell session, then will run: %q", expression)

	err := c.sendRequest(req)
	if err != nil {
		log.Printf("[DAP] Failed to send exec request: %v", err)
	}
	return err
}

// sendShellCommand sends a command to the active shell session.
func (c *DapClient) sendShellCommand(command string) error {
	c.shellMu.Lock()
	conn := c.shellConn
	c.shellMu.Unlock()

	if conn == nil {
		return fmt.Errorf("no shell connection")
	}

	// Reset terminal style before new command output.
	// Many shell commands (like ls --color, grep --color) output ANSI color codes
	// but don't always send a reset sequence (\x1b[0m) when they finish.
	// Without this reset, the next command's output would inherit the previous
	// command's styling, causing colors to "leak" between commands.
	c.Console.ResetStyle()

	log.Printf("[DAP] Sending to shell: %s", command)
	_, err := conn.Write([]byte(command + "\n"))
	return err
}

// handleRunInTerminal handles a RunInTerminalRequest from the DAP server.
// The buildx DAP uses this to set up exec sessions via Unix sockets.
func (c *DapClient) handleRunInTerminal(req *godap.RunInTerminalRequest) {
	log.Printf("[DAP] RunInTerminal request: kind=%s, args=%v", req.Arguments.Kind, req.Arguments.Args)

	// Look for socket path in args (format: "harbor dap attach <socket>")
	var socketPath string
	for i, arg := range req.Arguments.Args {
		if arg == "attach" && i+1 < len(req.Arguments.Args) {
			socketPath = req.Arguments.Args[i+1]
			break
		}
	}

	// Send acknowledgment response
	resp := &godap.RunInTerminalResponse{
		Response: godap.Response{
			ProtocolMessage: godap.ProtocolMessage{
				Seq:  c.nextSeq(),
				Type: "response",
			},
			RequestSeq: req.Seq,
			Success:    true,
			Command:    "runInTerminal",
		},
		Body: godap.RunInTerminalResponseBody{},
	}

	if err := c.sendRequest(resp); err != nil {
		log.Printf("[DAP] Failed to send RunInTerminal response: %v", err)
		return
	}

	// Connect to the shell socket
	if socketPath != "" {
		c.shellMu.Lock()
		if c.shellConn == nil {
			go c.connectShell(socketPath)
		}
		c.shellMu.Unlock()
	}
}

// connectShell connects to the shell socket and maintains a persistent connection.
func (c *DapClient) connectShell(socketPath string) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		log.Printf("[DAP] Failed to connect to shell socket: %v", err)
		c.appendToConsole(fmt.Sprintf("[Error] Failed to connect: %v\n", err))
		c.EvaluatePending = false
		return
	}

	c.shellMu.Lock()
	c.shellConn = conn
	c.shellMu.Unlock()

	log.Printf("[DAP] Connected to shell socket")

	// Read output continuously
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])

			// Check if shell is ready (prompt received)
			if !c.shellReady && (strings.Contains(chunk, "# ") || strings.Contains(chunk, "$ ")) {
				c.shellMu.Lock()
				c.shellReady = true
				c.shellMu.Unlock()
				log.Printf("[DAP] Shell ready")

				// If we have a pending command, send it now
				if c.pendingExpression != "" {
					cmd := c.pendingExpression
					c.pendingExpression = ""
					c.EvaluatePending = false
					go func() {
						if err := c.sendShellCommand(cmd); err != nil {
							log.Printf("[DAP] Failed to send pending command: %v", err)
						}
					}()
				}
			}

			// Append all output to console (including command echo and prompt)
			c.appendToConsole(chunk)
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("[DAP] Shell read error: %v", err)
			}
			break
		}
	}

	// Clean up
	c.shellMu.Lock()
	c.shellConn = nil
	c.shellReady = false
	c.shellMu.Unlock()
	log.Printf("[DAP] Shell connection closed")
}

// appendToConsole writes text to the console terminal emulator.
// The terminal handles ANSI escape sequences for colors and cursor movement.
func (c *DapClient) appendToConsole(chunk string) {
	c.Console.Write([]byte(chunk))
	select {
	case c.UpdateChan <- struct{}{}:
	default:
	}
}

// harborBuilderName is the name of the docker-container builder used for debugging.
const harborBuilderName = "harbor-debug"

// ensureDockerContainerBuilder ensures a docker-container driver builder exists.
// It returns the builder name to use for builds.
// If a builder named "harbor-debug" already exists with the docker-container driver,
// it will be reused. Otherwise, a new one is created.
func ensureDockerContainerBuilder(ctx context.Context, dockerCli command.Cli) (string, error) {
	txn, release, err := storeutil.GetStore(dockerCli)
	if err != nil {
		return "", fmt.Errorf("failed to get buildx store: %w", err)
	}
	defer release()

	// Check if our builder already exists
	ng, err := txn.NodeGroupByName(harborBuilderName)
	if err == nil && ng != nil && ng.Driver == "docker-container" {
		log.Printf("[Build] Using existing docker-container builder: %s", harborBuilderName)
		return harborBuilderName, nil
	}

	// Builder doesn't exist or has wrong driver, create a new one
	log.Printf("[Build] Creating docker-container builder: %s", harborBuilderName)

	b, err := builder.Create(ctx, txn, dockerCli, builder.CreateOpts{
		Name:   harborBuilderName,
		Driver: "docker-container",
		Use:    false, // Don't set as global default
	})
	if err != nil {
		return "", fmt.Errorf("failed to create docker-container builder: %w", err)
	}

	// Boot the builder to ensure it's ready
	log.Printf("[Build] Booting builder: %s", b.Name)
	if _, err := b.Boot(ctx); err != nil {
		return "", fmt.Errorf("failed to boot builder: %w", err)
	}

	return b.Name, nil
}

// runBuildWithHandler runs the Docker build using buildx with the DAP handler.
func (c *DapClient) runBuildWithHandler() {
	log.Println("[Build] Starting Docker build with DAP handler...")

	// Initialize Docker CLI
	dockerCli, err := command.NewDockerCli()
	if err != nil {
		log.Printf("[Build] Failed to create Docker CLI: %v", err)
		return
	}

	// Initialize with default options, respecting current context
	clientOpts := flags.NewClientOptions()
	if err := dockerCli.Initialize(clientOpts, command.WithInitializeClient(func(cli *command.DockerCli) (client.APIClient, error) {
		// Let it auto-detect from environment/config
		return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	})); err != nil {
		log.Printf("[Build] Failed to initialize Docker CLI: %v", err)
		return
	}

	// Ensure we have a docker-container builder for debugging
	builderName, err := ensureDockerContainerBuilder(c.ctx, dockerCli)
	if err != nil {
		log.Printf("[Build] Failed to ensure docker-container builder: %v", err)
		return
	}

	// Get the DAP handler from the adapter
	handler := c.adapter.Handler()

	// Create build options with our docker-container builder
	buildOpts := &commands.BuildOptions{
		ContextPath:    c.Params.Context,
		DockerfileName: c.Params.Dockerfile,
		Builder:        builderName,
		NoCache:        true,
		Pull:           true,
		ExportLoad:     true, // Load the image into docker
	}

	// Create a progress writer that logs to console
	printer, err := progress.NewPrinter(c.ctx, io.Discard, progressui.AutoMode)
	if err != nil {
		log.Printf("[Build] Failed to create progress printer: %v", err)
		return
	}

	// Run the build with the DAP handler
	log.Printf("[Build] Context: %s", buildOpts.ContextPath)
	log.Printf("[Build] Dockerfile: %s", buildOpts.DockerfileName)

	resp, _, err := commands.RunBuild(c.ctx, dockerCli, buildOpts, nil, printer, &handler)
	if err != nil {
		log.Printf("[Build] Build failed: %v", err)
	} else {
		log.Printf("[Build] Build completed successfully!")
		if resp != nil {
			log.Printf("[Build] Image ID: %s", resp.ExporterResponse["containerimage.digest"])
		}
	}

	// Wait for printer to finish
	if err := printer.Wait(); err != nil {
		log.Printf("[Build] Printer error: %v", err)
	}

	// Notify UI (only if context is still active)
	select {
	case <-c.ctx.Done():
		// Context cancelled, don't try to send
	case c.UpdateChan <- struct{}{}:
	default:
	}
}
