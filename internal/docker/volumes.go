package docker

import (
	"context"
	"sort"

	"github.com/docker/docker/api/types/volume"
)

// Volume represents a Docker volume with relevant information.
type Volume struct {
	Name       string
	Driver     string
	Mountpoint string
	CreatedAt  string
	Labels     map[string]string
}

// ListVolumes returns all volumes.
func (c *Client) ListVolumes(ctx context.Context) ([]Volume, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	resp, err := c.cli.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]Volume, 0, len(resp.Volumes))
	for _, vol := range resp.Volumes {
		result = append(result, Volume{
			Name:       vol.Name,
			Driver:     vol.Driver,
			Mountpoint: vol.Mountpoint,
			CreatedAt:  vol.CreatedAt,
			Labels:     vol.Labels,
		})
	}

	// Sort by name
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}
