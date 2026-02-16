package ports

import "context"

// BuilderService defines operations for building container images from source code.
// It bridges the gap between source control (Git) and containerization (Docker).
type BuilderService interface {
	// BuildImage clones a repository and builds a Docker image from it.
	// It returns the ID of the built image or an error.
	BuildImage(ctx context.Context, repoURL string, imageName string) (string, error)
}
