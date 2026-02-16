package ports

import (
	"context"
	"io"

	"github.com/melih/lighthouse-paas/internal/core/domain"
)

// ContainerService defines the contract for container orchestration.
// Implementations (e.g., DockerAdapter) must adhere to this interface
// to ensure the core domain remains agnostic of the underlying technology.
type ContainerService interface {
	// ListContainers returns a list of all containers, including stopped ones.
	ListContainers(ctx context.Context) ([]domain.Container, error)

	// StartContainer creates and starts a new container from the given image.
	// It returns the ID of the started container.
	StartContainer(ctx context.Context, image string) (string, error)

	// StopContainer stops a running container by its ID.
	StopContainer(ctx context.Context, id string) error

	// GetContainerLogs returns a stream of logs for the specified container.
	// The caller is responsible for closing the returned ReadCloser.
	GetContainerLogs(ctx context.Context, id string) (io.ReadCloser, error)
}
