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

// ProxyHandler manages reverse proxying for subdomains.
type ProxyHandler struct {
	service ports.ContainerService
}

// NewProxyHandler creates a new proxy handler.
func NewProxyHandler(service ports.ContainerService) *ProxyHandler {
	return &ProxyHandler{service: service}
}

// ProxyRequest intercepts requests to subdomains (e.g., app-name.localhost)
// and routes them to the corresponding container's internal IP.
func (h *ProxyHandler) ProxyRequest(c *fiber.Ctx) error {
	host := c.Hostname()

	// 1. Extract Subdomain
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return c.Next()
	}
	subdomain := parts[0]

	// Skip common subdomains or empty ones
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
	// This ensures the container receives a request with a Host header it expects (IP based),
	// avoiding "Host not recognized" errors from the application inside.
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = remote.Host
		req.URL.Host = remote.Host
		req.URL.Scheme = remote.Scheme
	}

	// Error Handler: Return standard BadGateway if connectivity fails
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(fmt.Sprintf("Proxy Info: target=%s error=%v", targetIP, err)))
	}

	// Fiber <-> Net/HTTP Adaptor
	return adaptor.HTTPHandler(proxy)(c)
}
