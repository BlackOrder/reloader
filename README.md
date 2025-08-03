# Reloader

A simple, robust Go package for watching file changes and triggering reload actions with debouncing support.

[![Go Reference](https://pkg.go.dev/badge/github.com/blackorder/reloader.svg)](https://pkg.go.dev/github.com/blackorder/reloader)
[![Go Report Card](https://goreportcard.com/badge/github.com/blackorder/reloader)](https://goreportcard.com/report/github.com/blackorder/reloader)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## Features

- 🔄 File change detection using fsnotify
- ⏱️ Configurable debouncing to prevent rapid successive triggers
- 🔁 Automatic retry mechanism with configurable delays
- 📝 Optional event and error logging callbacks
- 🛡️ Context-based cancellation support
- 📁 **Multi-file watching** across different directories
- 🔧 Self-monitoring convenience functions
- 🧪 Comprehensive test coverage

## Installation

```bash
go get github.com/blackorder/reloader
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/blackorder/reloader"
)

func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()

    config := reloader.Config{
        TargetFile: "/path/to/your/binary",
        OnChange: func() {
            fmt.Println("Reloading application...")
            // Your reload logic here
        },
        Debounce:   reloader.DefaultDebounce,
        RetryDelay: reloader.DefaultRetryDelay,
        OnEvent: func(msg string) {
            log.Printf("Event: %s", msg)
        },
        OnError: func(err error) {
            log.Printf("Error: %v", err)
        },
    }

    if err := reloader.Watch(ctx, config); err != nil {
        log.Fatal(err)
    }
}
```

## Configuration

### Config struct

The `Config` struct provides the following options:

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `OnChange` | `func()` | Callback function triggered when file changes | Required |
| `OnEvent` | `func(string)` | Optional callback for event logging | nil |
| `OnError` | `func(error)` | Optional callback for error logging | nil |
| `TargetFile` | `string` | Absolute path to the file to watch | Required |
| `Debounce` | `time.Duration` | Wait time before triggering reload after change detection | 3 seconds |
| `RetryDelay` | `time.Duration` | Wait time before recreating watcher on errors | 2 seconds |

### MultiConfig struct

The `MultiConfig` struct provides the following options:

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `OnChange` | `func(string)` | Callback function triggered when a file changes (receives the changed file path) | Required |
| `OnEvent` | `func(string)` | Optional callback for event logging | nil |
| `OnError` | `func(error)` | Optional callback for error logging | nil |
| `TargetFiles` | `[]string` | Absolute paths to the files to watch | Required |
| `Debounce` | `time.Duration` | Wait time before triggering reload after change detection | 3 seconds |
| `RetryDelay` | `time.Duration` | Wait time before recreating watcher on errors | 2 seconds |

### SelfMonitorConfig struct

The `SelfMonitorConfig` struct provides the following options:

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `OnReload` | `func()` | Callback function triggered when binary changes | Required |
| `OnEvent` | `func(string)` | Optional callback for event logging | nil |
| `OnError` | `func(error)` | Optional callback for error logging | nil |
| `Debounce` | `time.Duration` | Wait time before triggering reload after change detection | 3 seconds |
| `RetryDelay` | `time.Duration` | Wait time before recreating watcher on errors | 2 seconds |

## Advanced Usage

### Custom Debouncing

```go
config := reloader.Config{
    TargetFile: "/path/to/binary",
    OnChange:   reloadFunc,
    Debounce:   500 * time.Millisecond, // Fast response
}
```

### Self-Monitoring with SelfMonitor

For the common use case of monitoring your own binary, use the `SelfMonitor` convenience function:

```go
package main

import (
    "context"
    "log"
    "syscall"
    "time"

    "github.com/blackorder/reloader"
)

func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()

    // Simple self-monitoring with automatic executable detection
    go func() {
        err := reloader.SelfMonitor(ctx, reloader.SelfMonitorConfig{
            Debounce: 5 * time.Second,
            OnReload: func() {
                log.Println("Binary updated, triggering reload...")
                syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
            },
            OnEvent: func(msg string) { log.Println("Monitor:", msg) },
            OnError: func(err error) { log.Println("Monitor error:", err) },
        })
        if err != context.Canceled {
            log.Printf("Monitor failed: %v", err)
        }
    }()

    // Your application logic here...
}
```

### Multi-File Watching

Monitor multiple files across different directories with individual debouncing per file:

```go
package main

import (
    "context"
    "log"
    "strings"
    "time"

    "github.com/blackorder/reloader"
)

func main() {
    ctx := context.Background()
    
    files := []string{
        "/path/to/app1",
        "/path/to/app2", 
        "/etc/myapp/config.yaml",
    }

    config := reloader.MultiConfig{
        TargetFiles: files,
        OnChange: func(changedFile string) {
            log.Printf("File changed: %s", changedFile)
            // Handle different files differently
            if strings.HasSuffix(changedFile, ".yaml") {
                // Reload configuration
                reloadConfig()
            } else {
                // Restart binary
                restartProcess(changedFile)
            }
        },
        Debounce: reloader.DefaultDebounce,
        OnEvent:  func(msg string) { log.Println("Event:", msg) },
        OnError:  func(err error) { log.Println("Error:", err) },
    }

    if err := reloader.WatchMultiple(ctx, config); err != nil {
        log.Fatal(err)
    }
}
```

### Manual Self-Monitoring

For more control, you can manually specify the executable path:

```go
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

func main() {
    binaryPath, _ := os.Executable()
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()

    // Monitor own binary and trigger SIGHUP on changes
    go func() {
        _ = reloader.Watch(ctx, reloader.Config{
            TargetFile: binaryPath,
            Debounce:   reloader.DefaultDebounce,
            OnChange: func() {
                if reloadEnabled {
                    log.Println("Binary updated, sending SIGHUP for graceful reload")
                    _ = syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
                } else {
                    log.Println("Binary updated, but reloading is disabled")
                }
            },
            OnEvent: func(msg string) { log.Println("Watcher:", msg) },
            OnError: func(err error) { log.Println("Watcher error:", err) },
        })
    }()

    // Handle SIGHUP for graceful reloads
    sighup := make(chan os.Signal, 1)
    signal.Notify(sighup, syscall.SIGHUP)

    for {
        select {
        case <-ctx.Done():
            return
        case <-sighup:
            // Perform graceful reload logic here
            log.Println("Received SIGHUP, performing graceful reload...")
        }
    }
}
```

### Error Handling

```go
config := reloader.Config{
    TargetFile: "/path/to/binary",
    OnChange:   reloadFunc,
    OnError: func(err error) {
        // Custom error handling
        if errors.Is(err, os.ErrNotExist) {
            log.Println("Target file does not exist, waiting...")
        } else {
            log.Printf("Watcher error: %v", err)
        }
    },
}
```

### Event Monitoring

```go
config := reloader.Config{
    TargetFile: "/path/to/binary",
    OnChange:   reloadFunc,
    OnEvent: func(msg string) {
        // Custom event logging
        timestamp := time.Now().Format(time.RFC3339)
        fmt.Printf("[%s] %s\n", timestamp, msg)
    },
}
```

## How It Works

1. **Watcher Creation**: Creates a new fsnotify watcher for the directory containing the target file
2. **Change Detection**: Monitors for WRITE, CREATE, RENAME, and REMOVE events on the target file
3. **Debouncing**: Uses a timer to prevent rapid successive triggers when multiple changes occur
4. **Error Recovery**: Automatically recreates the watcher if errors occur
5. **Graceful Shutdown**: Responds to context cancellation for clean shutdown

## Event Types

The package watches for the following file system events:
- **WRITE**: File content modification
- **CREATE**: File creation
- **RENAME**: File renamed (includes moves)
- **REMOVE**: File deletion

## Error Handling

The package includes robust error handling:
- Automatic watcher recreation on errors
- Configurable retry delays
- Optional error callbacks for custom handling
- Context-based cancellation support

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built with [fsnotify](https://github.com/fsnotify/fsnotify) for cross-platform file system notifications
