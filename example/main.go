package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/blackorder/reloader"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: example <binary-to-watch>")
		fmt.Println("Example: example ./myapp")
		os.Exit(1)
	}

	binaryPath := os.Args[1]
	
	// Convert to absolute path
	absPath, err := filepath.Abs(binaryPath)
	if err != nil {
		log.Fatalf("Failed to get absolute path: %v", err)
	}

	// Set up context for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var cmd *exec.Cmd

	config := reloader.Config{
		TargetFile: absPath,
		OnChange: func() {
			log.Println("🔄 File change detected, restarting application...")
			
			// Stop the current process if running
			if cmd != nil && cmd.Process != nil {
				log.Println("⏹️  Stopping current process...")
				cmd.Process.Kill()
				cmd.Wait()
			}

			// Start the new process
			log.Println("🚀 Starting new process...")
			cmd = exec.Command(absPath)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			
			if err := cmd.Start(); err != nil {
				log.Printf("❌ Failed to start process: %v", err)
				return
			}
			
			log.Printf("✅ Process started with PID %d", cmd.Process.Pid)
		},
		Debounce:   1 * time.Second,
		RetryDelay: 2 * time.Second,
		OnEvent: func(msg string) {
			log.Printf("📡 %s", msg)
		},
		OnError: func(err error) {
			log.Printf("❌ Error: %v", err)
		},
	}

	// Start the initial process
	log.Printf("🚀 Starting initial process: %s", absPath)
	cmd = exec.Command(absPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start initial process: %v", err)
	}
	
	log.Printf("✅ Initial process started with PID %d", cmd.Process.Pid)

	// Start watching for changes
	log.Println("👀 Starting file watcher...")
	
	// Handle graceful shutdown
	go func() {
		<-ctx.Done()
		log.Println("🛑 Shutting down...")
		if cmd != nil && cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	if err := reloader.Watch(ctx, config); err != nil {
		if err != context.Canceled {
			log.Fatalf("Watcher error: %v", err)
		}
	}

	log.Println("👋 Goodbye!")
}
