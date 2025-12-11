package docker

import (
	"context"
	"runtime"
	"sync"

	"github.com/docker/docker/client"
)

const apiVersion = "1.51"

// Client wraps the Docker client with application-specific methods.
type Client struct {
	cli *client.Client
	mu  sync.RWMutex
}

// NewClient creates a new Docker client connected to the local Docker daemon.
// It auto-detects the socket location based on the operating system.
func NewClient() (*Client, error) {
	opts := []client.Opt{
		client.WithVersion(apiVersion),
	}

	// On Windows, use named pipe; on Unix systems, use the default socket
	if runtime.GOOS == "windows" {
		opts = append(opts, client.WithHost("npipe:////./pipe/docker_engine"))
	} else {
		// Linux and macOS use Unix socket
		opts = append(opts, client.WithHost("unix:///var/run/docker.sock"))
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, err
	}

	return &Client{cli: cli}, nil
}

// Close closes the Docker client connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cli.Close()
}

// Ping checks if the Docker daemon is accessible.
func (c *Client) Ping(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, err := c.cli.Ping(ctx)
	return err
}

// Raw returns the underlying Docker client for advanced operations.
func (c *Client) Raw() *client.Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cli
}
