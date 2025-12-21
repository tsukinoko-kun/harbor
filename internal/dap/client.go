package dap

import "log"

// DebugParams contains the parameters for starting a debug session.
// This struct is extensible - add new fields as needed.
type DebugParams struct {
	// Dockerfile is the path to the Dockerfile to debug.
	Dockerfile string
	// Context is the build context directory for the Dockerfile.
	Context string
}

// DapClient is the debug adapter protocol client.
// It manages the connection to a DAP server and provides
// a channel for notifying the UI of updates.
type DapClient struct {
	// Params contains the debug session parameters.
	Params DebugParams
	// UpdateChan is used to signal the UI that data has changed.
	// Send to this channel when the DAP server sends new data.
	UpdateChan chan struct{}
}

// Client is the global DAP client instance.
// It starts as nil and is set when a debug session is started.
var Client *DapClient

// NewClient creates a new DAP client with the given parameters.
// For now, this just logs the parameters instead of actually connecting.
func NewClient(params DebugParams) *DapClient {
	log.Printf("Starting debugger with parameters:")
	log.Printf("  Dockerfile: %s", params.Dockerfile)
	log.Printf("  Context: %s", params.Context)

	return &DapClient{
		Params:     params,
		UpdateChan: make(chan struct{}),
	}
}

// Close closes the DAP client connection and cleans up resources.
func (c *DapClient) Close() error {
	if c == nil {
		return nil
	}
	if c.UpdateChan != nil {
		close(c.UpdateChan)
	}
	return nil
}
