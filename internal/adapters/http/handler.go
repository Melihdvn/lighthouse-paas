package http

import (
	"github.com/gofiber/fiber/v2"
	"github.com/melih/lighthouse-paas/internal/core/ports"
)

type ContainerHandler struct {
	service ports.ContainerService
	builder ports.BuilderService
}

func NewContainerHandler(service ports.ContainerService, builder ports.BuilderService) *ContainerHandler {
	return &ContainerHandler{service: service, builder: builder}
}

func (h *ContainerHandler) ListContainers(c *fiber.Ctx) error {
	containers, err := h.service.ListContainers(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(containers)
}

type StartContainerRequest struct {
	Image   string `json:"image"`
	RepoURL string `json:"repo_url"` // New field for Git URL
}

func (h *ContainerHandler) StartContainer(c *fiber.Ctx) error {
	var req StartContainerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	var imageToRun string

	// Phase 6: Build from Source
	if req.RepoURL != "" {
		// Generate an image name based on repo name or random
		// For simplicity, let's use a hashed name or just "lighthouse-app" + timestamp
		// But to keep it simple, we'll ask user for image name OR generate one.
		// If user didn't provide image name but provided RepoURL, we generate one.
		if req.Image == "" {
			req.Image = "app-" + c.Params("id") // We don't have ID here yet.
			req.Image = "lighthouse-built-image" // simplistic fallback
		}
		imageToRun = req.Image

		// Trigger Build
		// Note: This is a blocking operation and might take time!
		// In a real system, we'd use a background job/worker.
		if _, err := h.builder.BuildImage(c.Context(), req.RepoURL, imageToRun); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Build failed: " + err.Error(),
			})
		}
	} else {
		// Legacy mode: Pull existing image
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
	// Fiber handles the closing of the stream automatically when passed to body? 
	// Actually no, we should probably handle it or let Fiber read it.
	// Fiber's c.SendStream is better, but since logs are io.ReadCloser (Reader), 
	// we can just return the stream if we set the content type.
	
	// For simplicity, let's just return it as plain text.
	c.Set("Content-Type", "text/plain")
	return c.SendStream(logs)
}
