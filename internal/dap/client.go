package dap

// DapClient is the debug adapter protocol client.
// It manages the connection to a DAP server and provides
// a channel for notifying the UI of updates.
type DapClient struct {
	// UpdateChan is used to signal the UI that data has changed.
	// Send to this channel when the DAP server sends new data.
	UpdateChan chan struct{}
}

// Client is the global DAP client instance.
// It starts as nil and is set when a debug session is started.
var Client *DapClient

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
