package http

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/melih/lighthouse-paas/internal/core/ports"
)

type ProxyHandler struct {
	service ports.ContainerService
}

func NewProxyHandler(service ports.ContainerService) *ProxyHandler {
	return &ProxyHandler{service: service}
}

// ProxyRequest handles requests to subdomains (e.g., app-name.localhost)
func (h *ProxyHandler) ProxyRequest(c *fiber.Ctx) error {
	host := c.Hostname() // e.g., "my-app.localhost"

	// 1. Extract Subdomain
	// Ideally we should configure the base domain. Assuming ".localhost" for now.
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		// No subdomain, let it pass to next handler (Dashboard/API)
		return c.Next()
	}
	subdomain := parts[0]

	// If subdomain is "www" or empty, skip
	if subdomain == "www" || subdomain == "" {
		return c.Next()
	}

	// 2. Find Container by Name (Subdomain)
	containers, err := h.service.ListContainers(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to list containers")
	}

	var targetIP string
	for _, container := range containers {
		if container.Name == subdomain {
			// Only proxy to running containers
			if container.State != "running" {
				continue
			}
			targetIP = container.IPAddress
			break
		}
	}

	if targetIP == "" {
		return c.Status(fiber.StatusNotFound).SendString(fmt.Sprintf("App '%s' not found or not running", subdomain))
	}

	// 3. Proxy the Request
	targetURL := fmt.Sprintf("http://%s", targetIP)
	remote, err := url.Parse(targetURL)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Invalid target URL")
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)
	
	// Custom Director: Rewrite Host header to target
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		// Crucial: Set Host to target (e.g., 127.0.0.1:32769)
		// This prevents the container from seeing 'strange_tu.localhost' which it might not recognize
		req.Host = remote.Host 
		req.URL.Host = remote.Host
		req.URL.Scheme = remote.Scheme
	}

	// Error Handler
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		fmt.Printf("PROXY ERROR: %v\n", err)
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(fmt.Sprintf("Proxy Info: target=%s error=%v", targetIP, err)))
	}
	
	// Fiber <-> Net/HTTP Adaptor
	return adaptor.HTTPHandler(proxy)(c)
}
