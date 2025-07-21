package reloader

import (
	"context"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Config lets each binary decide what to watch and how to react.
type Config struct {
	TargetFile string         // absolute path to the binary (or any file)
	Signal     syscall.Signal // e.g. syscall.SIGHUP
	Debounce   time.Duration  // wait before sending (default 3s)
	RetryDelay time.Duration  // wait before recreating watcher (default 2s)
	OnEvent    func(string)   // optional callback for logging
	OnError    func(error)    // optional callback for logging
}

// Watch blocks until ctx is done.
func Watch(ctx context.Context, cfg Config) error {
	if cfg.Debounce == 0 {
		cfg.Debounce = 3 * time.Second
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = 2 * time.Second
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
			time.Sleep(cfg.RetryDelay)
			continue
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
				_ = syscall.Kill(syscall.Getpid(), cfg.Signal)
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
