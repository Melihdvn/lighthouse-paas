package docker

import (
	"context"
	"fmt"
	
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// Adapter implements ports.ContainerService using Docker SDK
type Adapter struct {
	cli *client.Client
}

// NewAdapter creates a new Docker adapter instance
func NewAdapter() (*Adapter, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	return &Adapter{cli: cli}, nil
}

// ListContainers returns a list of running container names
func (a *Adapter) ListContainers(ctx context.Context) ([]string, error) {
	containers, err := a.cli.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var names []string
	for _, container := range containers {
		// Docker returns names with a leading slash (e.g., "/my-app")
		if len(container.Names) > 0 {
			names = append(names, container.Names[0][1:]) 
		}
	}
	return names, nil
}
