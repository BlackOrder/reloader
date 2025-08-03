package reloader

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	// DefaultDebounce is the default time to wait before triggering reload after change detection.
	DefaultDebounce = 3 * time.Second
	// DefaultRetryDelay is the default time to wait before recreating watcher on errors.
	DefaultRetryDelay = 2 * time.Second
)

// Config lets each binary decide what to watch and how to react.
type Config struct {
	OnChange   func()        // callback for reloading the binary
	OnEvent    func(string)  // optional callback for logging
	OnError    func(error)   // optional callback for logging
	TargetFile string        // absolute path to the binary (or any file)
	Debounce   time.Duration // wait before sending (default 3s)
	RetryDelay time.Duration // wait before recreating watcher (default 2s)
}

// Watch blocks until ctx is done.
func Watch(ctx context.Context, cfg Config) error {
	if cfg.Debounce == 0 {
		cfg.Debounce = DefaultDebounce
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = DefaultRetryDelay
	}
	if cfg.OnChange == nil {
		return errors.New("OnChange callback must be set")
	}

	for {
		w, err := fsnotify.NewWatcher()
		if err != nil {
			if cfg.OnError != nil {
				cfg.OnError(err)
			}
			select {
			case <-time.After(cfg.RetryDelay):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		dir := filepath.Dir(cfg.TargetFile)
		if err := w.Add(dir); err != nil {
			if cfg.OnError != nil {
				cfg.OnError(err)
			}
			_ = w.Close()
			select {
			case <-time.After(cfg.RetryDelay):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		if cfg.OnEvent != nil {
			cfg.OnEvent("watching " + dir)
		}

		debounce := time.NewTimer(cfg.Debounce)
		debounce.Stop() // idle

	loop:
		for {
			select {
			case <-ctx.Done():
				_ = w.Close()
				return ctx.Err()

			case ev := <-w.Events:
				if ev.Name == cfg.TargetFile &&
					(ev.Op&fsnotify.Write != 0 || ev.Op&fsnotify.Create != 0 ||
						ev.Op&fsnotify.Rename != 0 || ev.Op&fsnotify.Remove != 0) {
					if cfg.OnEvent != nil {
						cfg.OnEvent("change detected: " + ev.String())
					}
					debounce.Reset(cfg.Debounce)
				}

			case <-debounce.C:
				if cfg.OnEvent != nil {
					cfg.OnEvent("sending signal")
				}
				cfg.OnChange() // trigger reload

				debounce.Stop()

			case err := <-w.Errors:
				if err != nil && cfg.OnError != nil {
					cfg.OnError(err)
				}
				break loop // recreate watcher
			}
		}
		_ = w.Close()
	}
}

// SelfMonitor starts monitoring the current executable and calls the provided
// callback when changes are detected. This is a convenience function for the
// common pattern of self-monitoring applications.
//
// Example:
//
//	err := reloader.SelfMonitor(ctx, reloader.SelfMonitorConfig{
//	    Debounce: 5 * time.Second,
//	    OnReload: func() {
//	        log.Println("Binary updated, triggering reload...")
//	        syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
//	    },
//	})
func SelfMonitor(ctx context.Context, cfg SelfMonitorConfig) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}

	config := Config{
		TargetFile: executable,
		OnChange:   cfg.OnReload,
		Debounce:   cfg.Debounce,
		RetryDelay: cfg.RetryDelay,
		OnEvent:    cfg.OnEvent,
		OnError:    cfg.OnError,
	}

	return Watch(ctx, config)
}

// SelfMonitorConfig provides configuration for the SelfMonitor function.
type SelfMonitorConfig struct {
	OnReload   func()        // callback for reloading (required)
	OnEvent    func(string)  // optional callback for logging
	OnError    func(error)   // optional callback for logging
	Debounce   time.Duration // wait before sending (default 3s)
	RetryDelay time.Duration // wait before recreating watcher (default 2s)
}

// MultiConfig allows watching multiple files across different directories.
type MultiConfig struct {
	OnChange    func(string)  // callback with the file that changed
	OnEvent     func(string)  // optional callback for logging
	OnError     func(error)   // optional callback for logging
	TargetFiles []string      // absolute paths to the files to watch
	Debounce    time.Duration // wait before sending (default 3s)
	RetryDelay  time.Duration // wait before recreating watcher (default 2s)
}

// WatchMultiple blocks until ctx is done, watching multiple files.
func WatchMultiple(ctx context.Context, cfg MultiConfig) error {
	if cfg.Debounce == 0 {
		cfg.Debounce = DefaultDebounce
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = DefaultRetryDelay
	}
	if cfg.OnChange == nil {
		return errors.New("OnChange callback must be set")
	}
	if len(cfg.TargetFiles) == 0 {
		return errors.New("at least one target file must be specified")
	}

	// Group files by directory to minimize the number of watchers
	dirToFiles := make(map[string][]string)
	for _, file := range cfg.TargetFiles {
		dir := filepath.Dir(file)
		dirToFiles[dir] = append(dirToFiles[dir], file)
	}

	if cfg.OnEvent != nil {
		cfg.OnEvent(fmt.Sprintf("watching %d files across %d directories", len(cfg.TargetFiles), len(dirToFiles)))
	}

	for {
		w, err := fsnotify.NewWatcher()
		if err != nil {
			if cfg.OnError != nil {
				cfg.OnError(err)
			}
			select {
			case <-time.After(cfg.RetryDelay):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Add all directories to the watcher
		for dir := range dirToFiles {
			if err := w.Add(dir); err != nil {
				if cfg.OnError != nil {
					cfg.OnError(fmt.Errorf("failed to watch directory %s: %w", dir, err))
				}
				_ = w.Close()
				select {
				case <-time.After(cfg.RetryDelay):
					continue
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			if cfg.OnEvent != nil {
				cfg.OnEvent("watching directory: " + dir)
			}
		}

		// Channel to receive debounced events
		debouncedEvents := make(chan string, len(cfg.TargetFiles))

		// Active timers for debouncing
		activeTimers := make(map[string]*time.Timer)
		timerMutex := sync.Mutex{}

	loop:
		for {
			select {
			case <-ctx.Done():
				_ = w.Close()
				// Stop all active timers
				timerMutex.Lock()
				for _, timer := range activeTimers {
					timer.Stop()
				}
				timerMutex.Unlock()
				return ctx.Err()

			case ev := <-w.Events:
				// Check if this event is for one of our target files
				for _, targetFile := range cfg.TargetFiles {
					if ev.Name == targetFile &&
						(ev.Op&fsnotify.Write != 0 || ev.Op&fsnotify.Create != 0 ||
							ev.Op&fsnotify.Rename != 0 || ev.Op&fsnotify.Remove != 0) {
						if cfg.OnEvent != nil {
							cfg.OnEvent("change detected: " + ev.String())
						}

						// Handle debouncing for this specific file
						timerMutex.Lock()
						if existingTimer, exists := activeTimers[targetFile]; exists {
							existingTimer.Stop()
						}

						activeTimers[targetFile] = time.AfterFunc(cfg.Debounce, func() {
							debouncedEvents <- targetFile
							timerMutex.Lock()
							delete(activeTimers, targetFile)
							timerMutex.Unlock()
						})
						timerMutex.Unlock()
						break
					}
				}

			case file := <-debouncedEvents:
				if cfg.OnEvent != nil {
					cfg.OnEvent("sending signal for: " + file)
				}
				cfg.OnChange(file) // trigger reload with the specific file

			case err := <-w.Errors:
				if err != nil && cfg.OnError != nil {
					cfg.OnError(err)
				}
				break loop // recreate watcher
			}
		}
		_ = w.Close()
	}
}
