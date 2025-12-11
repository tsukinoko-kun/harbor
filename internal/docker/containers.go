package docker

import (
	"context"
	"sort"
	"strings"

	"github.com/docker/docker/api/types/container"
)

const composeProjectLabel = "com.docker.compose.project"

// Container represents a Docker container with relevant information.
type Container struct {
	ID      string
	Name    string
	Image   string
	Status  string
	State   string
	Project string // Compose project name, empty if standalone
}

// ContainerGroup represents a group of containers, either by project or standalone.
type ContainerGroup struct {
	Name       string
	Containers []Container
}

// ListContainers returns all containers (including stopped ones).
func (c *Client) ListContainers(ctx context.Context) ([]Container, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	containers, err := c.cli.ContainerList(ctx, container.ListOptions{
		All: true,
	})
	if err != nil {
		return nil, err
	}

	result := make([]Container, 0, len(containers))
	for _, ctr := range containers {
		name := ""
		if len(ctr.Names) > 0 {
			name = strings.TrimPrefix(ctr.Names[0], "/")
		}

		project := ""
		if p, ok := ctr.Labels[composeProjectLabel]; ok {
			project = p
		}

		result = append(result, Container{
			ID:      ctr.ID[:12],
			Name:    name,
			Image:   ctr.Image,
			Status:  ctr.Status,
			State:   ctr.State,
			Project: project,
		})
	}

	return result, nil
}

// ListContainersGrouped returns containers grouped by project.
// Standalone containers are grouped under an empty project name.
func (c *Client) ListContainersGrouped(ctx context.Context) ([]ContainerGroup, error) {
	containers, err := c.ListContainers(ctx)
	if err != nil {
		return nil, err
	}

	// Group by project
	groups := make(map[string][]Container)
	for _, ctr := range containers {
		groups[ctr.Project] = append(groups[ctr.Project], ctr)
	}

	// Convert to slice and sort
	result := make([]ContainerGroup, 0, len(groups))
	for name, ctrs := range groups {
		// Sort containers within group by name
		sort.Slice(ctrs, func(i, j int) bool {
			return ctrs[i].Name < ctrs[j].Name
		})
		result = append(result, ContainerGroup{
			Name:       name,
			Containers: ctrs,
		})
	}

	// Sort groups: named projects first (alphabetically), then standalone (empty name)
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name == "" {
			return false
		}
		if result[j].Name == "" {
			return true
		}
		return result[i].Name < result[j].Name
	})

	return result, nil
}
