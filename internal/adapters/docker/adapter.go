package docker

import (
	"context"
	"fmt"
	"time"
	
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/melih/lighthouse-paas/internal/core/domain"
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

// ListContainers returns a list of running containers with details
func (a *Adapter) ListContainers(ctx context.Context) ([]domain.Container, error) {
	containers, err := a.cli.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var result []domain.Container
	for _, c := range containers {
		// Use the first name if available, remove slash
		name := ""
		if len(c.Names) > 0 {
			name = c.Names[0][1:]
		}

		result = append(result, domain.Container{
			ID:     c.ID[:12], // Short ID
			Name:   name,
			Image:  c.Image,
			Status: c.Status,
			State:  c.State,
		})
	}
	return result, nil
}

// StartContainer creates and starts a container from a given image
func (a *Adapter) StartContainer(ctx context.Context, image string) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("context is nil")
	}
	// Context doesn't have a length property, so we remove that check.

	// 1. Image Pull (Ensure image exists)
	// In a real production system, we should handle auth and pull policy better.
	reader, err := a.cli.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()
	// Discard output to avoid filling memory, but user might want to see it logs in future
	io.Copy(os.Stdout, reader)

	// 2. Create Container
	resp, err := a.cli.ContainerCreate(ctx, &container.Config{
		Image: image,
	}, nil, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// 3. Start Container
	if err := a.cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	return resp.ID, nil
}

// StopContainer stops a running container
func (a *Adapter) StopContainer(ctx context.Context, id string) error {
	// Timeout can be configurable, but keeping it simple for now
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return a.cli.ContainerStop(ctx, id, container.StopOptions{})
}

// GetContainerLogs returns a stream of container logs
func (a *Adapter) GetContainerLogs(ctx context.Context, id string) (io.ReadCloser, error) {
	options := types.ContainerLogsOptions{
		ShowStdout: false,
		ShowStderr: true,
		Follow:     false, // Can be true for streaming
		Timestamps: true,
	}
	return a.cli.ContainerLogs(ctx, id, options)
}
