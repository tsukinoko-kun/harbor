package docker

import (
	"context"
	"sort"

	"github.com/docker/docker/api/types/network"
)

// Network represents a Docker network with relevant information.
type Network struct {
	ID     string
	Name   string
	Driver string
	Scope  string
}

// ListNetworks returns all networks.
func (c *Client) ListNetworks(ctx context.Context) ([]Network, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	networks, err := c.cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]Network, 0, len(networks))
	for _, net := range networks {
		id := net.ID
		if len(id) > 12 {
			id = id[:12]
		}

		result = append(result, Network{
			ID:     id,
			Name:   net.Name,
			Driver: net.Driver,
			Scope:  net.Scope,
		})
	}

	// Sort by name
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}
