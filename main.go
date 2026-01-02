package main

import (
	"log"
	"os"

	"gioui.org/app"

	"github.com/tsukinoko-kun/harbor/internal/config"
	"github.com/tsukinoko-kun/harbor/internal/dap"
	"github.com/tsukinoko-kun/harbor/internal/docker"
	"github.com/tsukinoko-kun/harbor/internal/env"
	"github.com/tsukinoko-kun/harbor/internal/ui"
)

func main() {
	// On macOS/Linux: load shell environment for Finder/launcher compatibility.
	// When launched from Finder (macOS) or desktop launchers (Linux),
	// shell environment variables like PATH are not inherited.
	// This ensures docker and other tools can be found.
	if shellEnv := env.LoadShellEnvironment(); len(shellEnv) > 0 {
		for k, v := range shellEnv {
			if os.Getenv(k) == "" { // Don't override existing vars
				os.Setenv(k, v)
			}
		}
	}

	// Ensure Docker bin directories are in PATH for credential helpers
	// (docker-credential-osxkeychain, etc.). This is a fallback in case
	// shell environment loading didn't include them.
	env.EnsureDockerInPath()

	// Load configuration
	settings, err := config.Load()
	if err != nil {
		log.Printf("Failed to load config: %v", err)
		os.Exit(1)
	}

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
		defer dap.Client.Close()

		application := ui.NewApp(dockerClient, settings)
		if err := application.Run(); err != nil {
			log.Printf("Application error: %v", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()

	// app.Main() must be called from the main goroutine
	app.Main()
}
