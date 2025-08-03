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

// Simple example using the SelfMonitor convenience function
func main() {
	log.Println("🚀 Starting service with self-monitoring...")

	// Create context for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Feature flag to enable/disable reloading
	reloadEnabled := true

	// Start self-monitoring in a goroutine using the convenience function
	go func() {
		err := reloader.SelfMonitor(ctx, reloader.SelfMonitorConfig{
			Debounce: 5 * time.Second,
			OnReload: func() {
				if reloadEnabled {
					log.Println("🔄 Binary updated, sending SIGHUP for graceful reload")
					if err := syscall.Kill(syscall.Getpid(), syscall.SIGHUP); err != nil {
						log.Printf("❌ Failed to send SIGHUP: %v", err)
					}
				} else {
					log.Println("🔄 Binary updated, but reloading is disabled")
				}
			},
			OnEvent: func(msg string) {
				log.Printf("📡 Monitor: %s", msg)
			},
			OnError: func(err error) {
				log.Printf("❌ Monitor error: %v", err)
			},
		})
		if err != nil && err != context.Canceled {
			log.Printf("❌ Self-monitor failed: %v", err)
		}
	}()

	// Handle SIGHUP for graceful reloads
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)

	// Simulate work
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	log.Println("✅ Service started. Update the binary to trigger a reload.")

	for {
		select {
		case <-ctx.Done():
			log.Println("🛑 Shutting down...")
			return

		case <-sighup:
			log.Println("🔄 Received SIGHUP, performing graceful reload...")
			// Toggle the reload feature as an example
			reloadEnabled = !reloadEnabled
			log.Printf("🔧 Reload feature is now: %v", reloadEnabled)

		case <-ticker.C:
			log.Println("💓 Service heartbeat...")
		}
	}
}
