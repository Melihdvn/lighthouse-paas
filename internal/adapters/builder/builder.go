package builder

import (
	"context"
	"fmt"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/go-git/go-git/v5"
)

type Adapter struct {
	cli *client.Client
}

func NewBuilderAdapter() (*Adapter, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	return &Adapter{cli: cli}, nil
}

// BuildImage clones a repo and builds a Docker image
func (a *Adapter) BuildImage(ctx context.Context, repoURL string, imageName string) (string, error) {
	// 1. Create temporary directory
	tmpDir, err := os.MkdirTemp("", "lighthouse-build-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir) // Clean up after build

	// 2. Clone Repository
	fmt.Printf("Cloning %s into %s...\n", repoURL, tmpDir)
	_, err = git.PlainCloneContext(ctx, tmpDir, false, &git.CloneOptions{
		URL:      repoURL,
		Progress: os.Stdout,
		Depth:    1, // Shallow clone for speed
	})
	if err != nil {
		return "", fmt.Errorf("failed to clone repo: %w", err)
	}

	// 3. Create Build Context (Tar)
	tar, err := archive.TarWithOptions(tmpDir, &archive.TarOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create build context: %w", err)
	}

	// 4. Build Docker Image
	fmt.Printf("Building Docker image: %s...\n", imageName)
	resp, err := a.cli.ImageBuild(ctx, tar, types.ImageBuildOptions{
		Tags:       []string{imageName},
		Dockerfile: "Dockerfile",
		Remove:     true, // Remove intermediate containers
	})
	if err != nil {
		return "", fmt.Errorf("failed to build image: %w", err)
	}
	defer resp.Body.Close()

	// Wait for build to complete (reading body completely)
	// We are discarding output here, but could stream it to user.
	// io.Copy(os.Stdout, resp.Body)
	// Using a buffer or discard is necessary to let the build finish.
	// For now, let's just drain it.
	buf := make([]byte, 1024)
	for {
		_, err := resp.Body.Read(buf)
		if err != nil {
			break
		}
	}

	return imageName, nil
}
