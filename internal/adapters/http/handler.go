package http

import (
	"github.com/gofiber/fiber/v2"
	"github.com/melih/lighthouse-paas/internal/core/ports"
)

// ContainerHandler manages HTTP requests for container operations.
type ContainerHandler struct {
	service ports.ContainerService
	builder ports.BuilderService
}

// NewContainerHandler creates a new handler with required dependencies.
func NewContainerHandler(service ports.ContainerService, builder ports.BuilderService) *ContainerHandler {
	return &ContainerHandler{service: service, builder: builder}
}

// ListContainers returns a JSON list of all containers.
// GET /api/v1/containers
func (h *ContainerHandler) ListContainers(c *fiber.Ctx) error {
	containers, err := h.service.ListContainers(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(containers)
}

// StartContainerRequest defines the payload for starting a container.
type StartContainerRequest struct {
	Image   string `json:"image"`
	RepoURL string `json:"repo_url"` // Optional: Git URL for source-based deployment
}

// StartContainer creates and starts a new container.
// It supports two modes:
// 1. Image-based: Starts a container from an existing Docker image.
// 2. Source-based: Builds a Docker image from a Git repository and then starts it.
// POST /api/v1/containers
func (h *ContainerHandler) StartContainer(c *fiber.Ctx) error {
	var req StartContainerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	var imageToRun string

	// Source-based Deployment
	if req.RepoURL != "" {
		// If no image name provided, fallback to a default name
		if req.Image == "" {
			req.Image = "lighthouse-built-image"
		}
		imageToRun = req.Image

		// Build the image from source
		if _, err := h.builder.BuildImage(c.Context(), req.RepoURL, imageToRun); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Build failed: " + err.Error(),
			})
		}
	} else {
		// Image-based Deployment
		if req.Image == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Image name or Repo URL is required",
			})
		}
		imageToRun = req.Image
	}

	containerID, err := h.service.StartContainer(c.Context(), imageToRun)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":    containerID,
		"image": req.Image,
	})
}

// StopContainer terminates a running container.
// DELETE /api/v1/containers/:id
func (h *ContainerHandler) StopContainer(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Container ID is required",
		})
	}

	if err := h.service.StopContainer(c.Context(), id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.SendStatus(fiber.StatusOK)
}

// GetContainerLogs returns the logs of a container.
// GET /api/v1/containers/:id/logs
func (h *ContainerHandler) GetContainerLogs(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Container ID is required",
		})
	}

	logs, err := h.service.GetContainerLogs(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Stream the logs back to the client as plain text
	c.Set("Content-Type", "text/plain")
	return c.SendStream(logs)
}
