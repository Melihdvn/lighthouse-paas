package docker

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/melih/lighthouse-paas/internal/core/domain"
	"net"
)

const (
	defaultStopTimeout = 10 * time.Second
)

// Adapter implements ports.ContainerService using the official Docker SDK.
// It manages the lifecycle of containers including starting, stopping, and inspecting.
type Adapter struct {
	cli *client.Client
}

// NewAdapter creates a new instance of the Docker Adapter.
// It attempts to initialize the Docker client from environment variables.
func NewAdapter() (*Adapter, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	return &Adapter{cli: cli}, nil
}

// ListContainers retrieves a list of all containers (running and stopped).
// It maps the Docker API response to the domain Container entity.
func (a *Adapter) ListContainers(ctx context.Context) ([]domain.Container, error) {
	containers, err := a.cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var result []domain.Container
	for _, c := range containers {
		// Extract container name
		name := ""
		if len(c.Names) > 0 {
			name = c.Names[0][1:]
		}

		// Determine accessible address (Host Port)
		hostPort := ""

		// If there are port mappings, pick the first one that maps to a public port.
		// Since we now support variable internal ports (80, 8080, 3000),
		// any mapped port is a valid candidate for the proxy.
		if len(c.Ports) > 0 {
			// Iterate to find a valid public port
			for _, p := range c.Ports {
				if p.PublicPort != 0 {
					hostPort = fmt.Sprintf("127.0.0.1:%d", p.PublicPort)
					break
				}
			}
		}

		// Fallback (rare): use network IP if no host binding exists
		if hostPort == "" && len(c.NetworkSettings.Networks) > 0 {
			for _, network := range c.NetworkSettings.Networks {
				// We don't know the internal port easily here without inspection,
				// but for bridge mode, we usually rely on the loopback mapping above.
				// This is a last resort fallback.
				hostPort = network.IPAddress + ":8080"
				break
			}
		}

		result = append(result, domain.Container{
			ID:        c.ID[:12],
			Name:      name,
			Image:     c.Image,
			Status:    c.Status,
			State:     c.State,
			IPAddress: hostPort,
		})
	}
	return result, nil
}

// StartContainer ensures the image exists and starts a new container.
// It automatically detects the exposed port from the image configuration
// and maps it to a random available port on the host.
func (a *Adapter) StartContainer(ctx context.Context, image string) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("context is nil")
	}

	// 1. Ensure image exists locally or pull it
	// We also inspect the image to find the Exposed Port
	details, _, err := a.cli.ImageInspectWithRaw(ctx, image)
	if err != nil {
		if client.IsErrNotFound(err) {
			fmt.Printf("Pulling image %s...\n", image)
			reader, pullErr := a.cli.ImagePull(ctx, image, types.ImagePullOptions{})
			if pullErr != nil {
				return "", fmt.Errorf("failed to pull image: %w", pullErr)
			}
			defer reader.Close()
			// Wait for pull to complete
			io.Copy(io.Discard, reader)

			// Re-inspect after pull
			details, _, err = a.cli.ImageInspectWithRaw(ctx, image)
			if err != nil {
				return "", fmt.Errorf("failed to inspect image after pull: %w", err)
			}
		} else {
			return "", fmt.Errorf("failed to inspect image: %w", err)
		}
	}

	// 2. Detect Exposed Port
	// Default to 8080 if none found
	var targetPort nat.Port = "8080/tcp"

	// Strategy:
	// - If specific ports are exposed, pick the first one.
	// - Prefer 80 or 8080 if multiple exist.
	if len(details.Config.ExposedPorts) > 0 {
		// Convert to list for easier checking
		var ports []string
		for p := range details.Config.ExposedPorts {
			ports = append(ports, string(p))
		}

		// Check for preferred ports
		found := false
		for _, p := range ports {
			if p == "80/tcp" || p == "8080/tcp" || p == "3000/tcp" {
				targetPort = nat.Port(p)
				found = true
				break
			}
		}

		// If no preferred port found, use the first one
		if !found && len(ports) > 0 {
			targetPort = nat.Port(ports[0])
		}
	}

	fmt.Printf("Auto-detected container port: %s\n", targetPort)

	// 3. Configure container with dynamic port binding
	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			targetPort: []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: "0", // Random available host port
				},
			},
		},
	}

	// Create request
	resp, err := a.cli.ContainerCreate(ctx, &container.Config{
		Image: image,
		ExposedPorts: nat.PortSet{
			targetPort: struct{}{},
		},
	}, hostConfig, nil, nil, "")

	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// 4. Start Container
	if err := a.cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	// 5. Wait for Port to be Ready (Health Check)
	// We try to connect to the random host port to see if it's accepting connections.

	// First, we need to inspect the container again to find out which random port was assigned.
	inspect, err := a.cli.ContainerInspect(ctx, resp.ID)
	if err != nil {
		return resp.ID, nil
	}

	// Find the binding
	var assignedPort string
	if bindings, ok := inspect.NetworkSettings.Ports[targetPort]; ok && len(bindings) > 0 {
		assignedPort = bindings[0].HostPort
	}

	if assignedPort != "" {
		fmt.Printf("Waiting for container %s to be ready on port %s...\n", resp.ID[:12], assignedPort)
		// Simple retry loop
		for i := 0; i < 20; i++ {
			conn, err := net.DialTimeout("tcp", "127.0.0.1:"+assignedPort, 500*time.Millisecond)
			if err == nil {
				conn.Close()
				fmt.Println("Container is ready!")
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
	}

	return resp.ID, nil
}

// StopContainer terminates a running container gracefully with a timeout.
func (a *Adapter) StopContainer(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, defaultStopTimeout)
	defer cancel()
	return a.cli.ContainerStop(ctx, id, container.StopOptions{})
}

// GetContainerLogs returns a read-only stream of the container's logs (stdout and stderr).
func (a *Adapter) GetContainerLogs(ctx context.Context, id string) (io.ReadCloser, error) {
	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     false,
		Timestamps: true,
	}
	return a.cli.ContainerLogs(ctx, id, options)
}
