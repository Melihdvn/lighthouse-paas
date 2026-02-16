package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/melih/lighthouse-paas/internal/adapters/docker"
	"github.com/melih/lighthouse-paas/internal/adapters/http"
)

func main() {
	// 1. Initialize Adapters (Infrastructure)
	dockerAdapter, err := docker.NewAdapter()
	if err != nil {
		log.Fatalf("Failed to initialize Docker adapter: %v", err)
	}

	// 2. Initialize HTTP Handlers (Interface Adapters)
	// Dependency Injection: Injecting the Docker Adapter (which implements ContainerService)
	// into the HTTP Handler.
	containerHandler := http.NewContainerHandler(dockerAdapter)

	// 3. Setup Framework (Fiber)
	app := fiber.New()

	// 4. Define Routes
	api := app.Group("/api")
	v1 := api.Group("/v1")

	// Routes for Container operations
	containers := v1.Group("/containers")
	containers.Get("/", containerHandler.ListContainers)
	containers.Post("/", containerHandler.StartContainer)
	containers.Delete("/:id", containerHandler.StopContainer)
	containers.Get("/:id/logs", containerHandler.GetContainerLogs)

	// 5. Start Server
	log.Println("Server starting on :3000")
	if err := app.Listen(":3000"); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}