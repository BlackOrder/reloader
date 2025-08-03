package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/blackorder/reloader"
)

const (
	// File extensions for configuration files
	extYAML = ".yaml"
	extYML  = ".yml"
	extJSON = ".json"
	extTOML = ".toml"
	extCONF = ".conf"

	// Minimum number of command line arguments required
	minArgs = 2

	// Default retry delay in seconds
	defaultRetryDelaySeconds = 2
)

func main() {
	if len(os.Args) < minArgs {
		fmt.Println("Usage: example-multi <file1> [file2] [file3] ...")
		fmt.Println("Example: example-multi ./app1 ./app2 ./config.yaml")
		os.Exit(1)
	}

	// Convert all paths to absolute paths and validate
	var targetFiles []string
	for _, arg := range os.Args[1:] {
		absPath, err := filepath.Abs(arg)
		if err != nil {
			log.Fatalf("Failed to get absolute path for %s: %v", arg, err)
		}

		// Security: Validate the file exists
		if _, err := os.Stat(absPath); err != nil {
			log.Fatalf("Failed to stat file %s: %v", absPath, err)
		}

		targetFiles = append(targetFiles, absPath)
	}

	// Set up context for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Map to track running processes
	runningProcesses := make(map[string]*exec.Cmd)

	config := reloader.MultiConfig{
		TargetFiles: targetFiles,
		OnChange: func(changedFile string) {
			log.Printf("🔄 File change detected: %s", changedFile)

			// Determine action based on file extension
			ext := strings.ToLower(filepath.Ext(changedFile))
			base := filepath.Base(changedFile)

			switch ext {
			case extYAML, extYML, extJSON, extTOML, extCONF:
				log.Printf("📝 Configuration file %s changed, notifying all processes...", base)
				// In a real scenario, you might reload config for all processes
				for file, cmd := range runningProcesses {
					if cmd != nil && cmd.Process != nil {
						log.Printf("🔄 Sending SIGHUP to process for %s", filepath.Base(file))
						// Send SIGHUP to the process for graceful config reload
						if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
							log.Printf("❌ Failed to send SIGHUP to %s: %v", filepath.Base(file), err)
						}
					}
				}

			default:
				// Restart the specific binary that changed
				if cmd, exists := runningProcesses[changedFile]; exists && cmd != nil && cmd.Process != nil {
					log.Printf("⏹️  Stopping process for %s...", base)
					if err := cmd.Process.Kill(); err != nil {
						log.Printf("⚠️  Error killing process for %s: %v", base, err)
					}
					if err := cmd.Wait(); err != nil {
						log.Printf("⚠️  Error waiting for process %s: %v", base, err)
					}
				}

				log.Printf("🚀 Starting new process for %s...", base)
				newCmd := exec.Command(changedFile)
				newCmd.Stdout = os.Stdout
				newCmd.Stderr = os.Stderr

				if err := newCmd.Start(); err != nil {
					log.Printf("❌ Failed to start %s: %v", base, err)
					return
				}

				runningProcesses[changedFile] = newCmd
				log.Printf("✅ Process started for %s with PID %d", base, newCmd.Process.Pid)
			}
		},
		Debounce:   1 * time.Second,
		RetryDelay: defaultRetryDelaySeconds * time.Second,
		OnEvent: func(msg string) {
			log.Printf("📡 %s", msg)
		},
		OnError: func(err error) {
			log.Printf("❌ Error: %v", err)
		},
	}

	// Start initial processes for executable files
	for _, file := range targetFiles {
		ext := strings.ToLower(filepath.Ext(file))
		base := filepath.Base(file)

		// Skip configuration files
		if ext == extYAML || ext == extYML || ext == extJSON || ext == extTOML || ext == extCONF {
			log.Printf("📝 Monitoring configuration file: %s", base)
			continue
		}

		// Start executable files
		log.Printf("🚀 Starting initial process for %s...", base)
		// #nosec G204 - This is intentional for a reloader example; file path is validated above
		cmd := exec.Command(file)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			log.Printf("❌ Failed to start %s: %v", base, err)
			continue
		}

		runningProcesses[file] = cmd
		log.Printf("✅ Initial process started for %s with PID %d", base, cmd.Process.Pid)
	}

	// Start watching for changes
	log.Printf("👀 Starting multi-file watcher for %d files...", len(targetFiles))

	// Handle graceful shutdown
	go func() {
		<-ctx.Done()
		log.Println("🛑 Shutting down...")
		for file, cmd := range runningProcesses {
			if cmd != nil && cmd.Process != nil {
				log.Printf("⏹️  Stopping process for %s...", filepath.Base(file))
				if err := cmd.Process.Kill(); err != nil {
					log.Printf("⚠️  Error killing process for %s: %v", filepath.Base(file), err)
				}
			}
		}
	}()

	if err := reloader.WatchMultiple(ctx, config); err != nil {
		if err != context.Canceled {
			log.Printf("❌ Watcher error: %v", err)
		}
	}

	log.Println("👋 Goodbye!")
}
