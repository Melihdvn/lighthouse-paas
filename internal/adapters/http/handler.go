package http

import (
	"github.com/gofiber/fiber/v2"
	"github.com/melih/lighthouse-paas/internal/core/ports"
)

type ContainerHandler struct {
	service ports.ContainerService
}

func NewContainerHandler(service ports.ContainerService) *ContainerHandler {
	return &ContainerHandler{service: service}
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
	Image string `json:"image"`
}

func (h *ContainerHandler) StartContainer(c *fiber.Ctx) error {
	var req StartContainerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Image == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Image name is required",
		})
	}

	containerID, err := h.service.StartContainer(c.Context(), req.Image)
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
