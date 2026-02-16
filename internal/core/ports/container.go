package ports

import "context"

// ContainerService defines the core operations for managing containers.
// This interface allows us to switch between Docker, Podman, or Kubernetes
// without changing the business logic.
type ContainerService interface {
	ListContainers(ctx context.Context) ([]string, error)
	// We will add Start/Stop later
}
