package main

import (
	"log"
	"os"

	"gioui.org/app"

	"github.com/tsukinoko-kun/harbor/internal/docker"
	"github.com/tsukinoko-kun/harbor/internal/ui"
)

func main() {
	// Initialize Docker client
	dockerClient, err := docker.NewClient()
	if err != nil {
		log.Printf("Failed to connect to Docker: %v", err)
		log.Println("Make sure Docker is running and accessible.")
		os.Exit(1)
	}

	// Run the application in a goroutine
	go func() {
		defer dockerClient.Close()

		application := ui.NewApp(dockerClient)
		if err := application.Run(); err != nil {
			log.Printf("Application error: %v", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()

	// app.Main() must be called from the main goroutine
	app.Main()
}
