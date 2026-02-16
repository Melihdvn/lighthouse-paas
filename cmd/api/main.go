// Lighthouse PaaS API Entry Point
// This file initializes the application infrastructure, dependency injection, and HTTP server.
package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/melih/lighthouse-paas/internal/adapters/builder"
	"github.com/melih/lighthouse-paas/internal/adapters/docker"
	"github.com/melih/lighthouse-paas/internal/adapters/http"
)

func main() {
	// Initialize Docker Adapter
	dockerAdapter, err := docker.NewAdapter()
	if err != nil {
		log.Fatalf("Failed to initialize Docker adapter: %v", err)
	}

	// Initialize Builder Adapter
	builderAdapter, err := builder.NewBuilderAdapter()
	if err != nil {
		log.Fatalf("Failed to initialize Builder adapter: %v", err)
	}

	// Initialize Interface Adapters (HTTP Handlers)
	containerHandler := http.NewContainerHandler(dockerAdapter, builderAdapter)
	proxyHandler := http.NewProxyHandler(dockerAdapter)

	// Setup Fiber Framework
	app := fiber.New()

	// Middleware Registration
	// Proxy Middleware must be registered BEFORE Static file serving.
	// This ensures subdomains (e.g., app.localhost) are intercepted and proxied,
	// while the root domain falls through to the dashboard.
	app.Use(proxyHandler.ProxyRequest)

	// Serve Static Dashboard
	app.Static("/", "./web")

	// API Routes
	api := app.Group("/api")
	v1 := api.Group("/v1")

	// Container Management Routes
	containers := v1.Group("/containers")
	containers.Get("/", containerHandler.ListContainers)
	containers.Post("/", containerHandler.StartContainer)
	containers.Delete("/:id", containerHandler.StopContainer)
	containers.Get("/:id/logs", containerHandler.GetContainerLogs)

	// Start Server
	log.Println("Server starting on :3000")
	if err := app.Listen(":3000"); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
