package ports

import (
	"context"
	"io"

	"github.com/melih/lighthouse-paas/internal/core/domain"
)

// ContainerService defines the core operations for managing containers.
// This interface allows us to switch between Docker, Podman, or Kubernetes
// without changing the business logic.
type ContainerService interface {
	ListContainers(ctx context.Context) ([]domain.Container, error)
	StartContainer(ctx context.Context, image string) (string, error)
	StopContainer(ctx context.Context, id string) error
	GetContainerLogs(ctx context.Context, id string) (io.ReadCloser, error)
}
