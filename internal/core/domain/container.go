package domain

// Container represents a container in the system (Docker, K8s, etc.)
type Container struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	Status  string `json:"status"`
	State   string `json:"state"` // running, exited, etc.
}
