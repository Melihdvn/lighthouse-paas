package domain

// Container represents a container in the system (Docker, K8s, etc.)
// Container represents a deployable application container instance.
// It maps to a Docker container but abstracts the specifics for the domain layer.
type Container struct {
	// ID is the unique identifier of the container (e.g., Docker ID)
	ID string `json:"id"`
	// Name is the human-readable name of the container (used for subdomain)
	Name string `json:"name"`
	// Image is the source image used to create the container
	Image string `json:"image"`
	// Status contains the detailed status string (e.g., "Up 2 hours")
	Status string `json:"status"`
	// State represents the coarse-grained state (e.g., "running", "exited")
	State string `json:"state"`
	// IPAddress is the accessible address for the reverse proxy (e.g., "127.0.0.1:32768")
	IPAddress string `json:"ip_address"`
}
