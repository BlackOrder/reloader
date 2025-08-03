package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/blackorder/reloader"
)

// This example demonstrates self-monitoring and graceful reloads using SIGHUP
// Similar to how you might implement it in a production service
func main() {
	// Get the path to current binary
	binaryPath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}

	log.Printf("🚀 Starting service, monitoring binary: %s", binaryPath)

	// Create context for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Channel to handle SIGHUP for graceful reloads
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)

	// Feature flag to enable/disable reloading
	reloadEnabled := true

	// Start the file watcher in a goroutine
	go func() {
		err := reloader.Watch(ctx, reloader.Config{
			TargetFile: binaryPath,
			Debounce:   5 * time.Second, // Same as your config
			OnChange: func() {
				if reloadEnabled {
					log.Printf("🔄 Binary change detected, sending SIGHUP for graceful reload")
					// Send SIGHUP to ourselves for graceful reload
					if err := syscall.Kill(syscall.Getpid(), syscall.SIGHUP); err != nil {
						log.Printf("❌ Failed to send SIGHUP: %v", err)
					}
				} else {
					log.Printf("🔄 Binary change detected, but reloading is disabled")
				}
			},
			OnEvent: func(msg string) {
				log.Printf("📡 Watcher: %s", msg)
			},
			OnError: func(err error) {
				log.Printf("❌ Watcher error: %v", err)
			},
		})
		if err != nil && err != context.Canceled {
			log.Printf("❌ Watcher failed: %v", err)
		}
	}()

	// Simulate some work
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	log.Println("✅ Service started. Press Ctrl+C to stop, or update the binary to trigger reload")

	for {
		select {
		case <-ctx.Done():
			log.Println("🛑 Received shutdown signal, stopping service...")
			return

		case <-sighup:
			log.Println("🔄 Received SIGHUP, performing graceful reload...")
			// In a real service, you would:
			// 1. Stop accepting new requests
			// 2. Finish processing current requests
			// 3. Reload configuration
			// 4. Restart components as needed
			// 5. Resume accepting requests
			
			// For this example, we'll just toggle the reload feature
			reloadEnabled = !reloadEnabled
			log.Printf("🔧 Reload feature is now: %v", reloadEnabled)

		case <-ticker.C:
			log.Println("💓 Service is running...")
		}
	}
}
