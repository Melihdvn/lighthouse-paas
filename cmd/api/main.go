package main

import (
	"context"
	"fmt"
	"log"

	"github.com/melih/antigravity/internal/adapters/docker"
)

func main() {
	// 1. Adapter'ı başlat (Docker'a bağlan)
	dockerAdapter, err := docker.NewAdapter()
	if err != nil {
		log.Fatalf("Failed to initialize Docker adapter: %v", err)
	}

	// 2. Servisi Test Et
	containers, err := dockerAdapter.ListContainers(context.Background())
	if err != nil {
		log.Printf("Error listing containers: %v", err)
	} else {
		fmt.Println("Çalışan Konteynerler:", containers)
	}
}