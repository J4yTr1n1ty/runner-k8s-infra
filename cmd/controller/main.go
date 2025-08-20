package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/J4yTr1n1ty/runner-k8s-infra/pkg/config"
	"github.com/J4yTr1n1ty/runner-k8s-infra/pkg/controller"
	"k8s.io/klog/v2"
)

func main() {
	// Parse flags
	var configPath string
	flag.StringVar(&configPath, "config", "", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		klog.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		klog.Fatalf("Invalid configuration: %v", err)
	}

	// Create controller
	ctrl, err := controller.NewController(cfg)
	if err != nil {
		klog.Fatalf("Failed to create controller: %v", err)
	}

	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		klog.Info("Received shutdown signal")
		cancel()
	}()

	// Start controller
	if err := ctrl.Start(ctx); err != nil {
		klog.Fatalf("Controller failed: %v", err)
	}
}
