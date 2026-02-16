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
	"github.com/docker/go-connections/nat"
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

		// Get Port Mapping (Host Port)
		// We prioritize the mapped port over internal IP for connectivity from host
		hostPort := ""
		if len(c.Ports) > 0 {
			// Find the mapping for 8080/tcp
			for _, p := range c.Ports {
				if p.PrivatePort == 8080 {
					hostPort = fmt.Sprintf("127.0.0.1:%d", p.PublicPort)
					break
				}
			}
			// Fallback if 8080 is not found but other ports exist
			if hostPort == "" {
				hostPort = fmt.Sprintf("127.0.0.1:%d", c.Ports[0].PublicPort)
			}
		}
		
		// Fallback to internal IP if no ports mapped (should not happen with new logic)
		if hostPort == "" {
			for _, network := range c.NetworkSettings.Networks {
				hostPort = network.IPAddress + ":8080"
				break 
			}
		}

		result = append(result, domain.Container{
			ID:        c.ID[:12], // Short ID
			Name:      name,
			Image:     c.Image,
			Status:    c.Status,
			State:     c.State,
			IPAddress: hostPort, // We are reusing this field to mean "Target Address"
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

	// 1. Check if image exists locally
	_, _, err := a.cli.ImageInspectWithRaw(ctx, image)
	if err != nil {
		if client.IsErrNotFound(err) {
			// Image not found locally, try to pull
			fmt.Printf("Pulling image %s...\n", image)
			reader, pullErr := a.cli.ImagePull(ctx, image, types.ImagePullOptions{})
			if pullErr != nil {
				return "", fmt.Errorf("failed to pull image: %w", pullErr)
			}
			defer reader.Close()
			io.Copy(os.Stdout, reader)
		} else {
			return "", fmt.Errorf("failed to inspect image: %w", err)
		}
	} else {
		fmt.Printf("Image %s found locally, skipping pull.\n", image)
	}

	// 2. Create Container with Port Binding
	// We bind container's 8080 to a random host port
	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			"8080/tcp": []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: "0", // Random available port
				},
			},
		},
	}

	resp, err := a.cli.ContainerCreate(ctx, &container.Config{
		Image: image,
		ExposedPorts: nat.PortSet{
			"8080/tcp": struct{}{},
		},
	}, hostConfig, nil, nil, "")
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
