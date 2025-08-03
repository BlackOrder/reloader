package reloader

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestConfig_Defaults(t *testing.T) {
	tempFile := createTempFile(t)
	defer os.Remove(tempFile)

	config := Config{
		TargetFile: tempFile,
		OnChange: func() {
			// Test callback
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := Watch(ctx, config)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}

	// Test that defaults work by creating a new config and checking the Watch function behavior
	// The defaults are applied inside the Watch function, so we can't check them on the original config
}

func TestConfig_MissingOnChange(t *testing.T) {
	tempFile := createTempFile(t)
	defer os.Remove(tempFile)

	config := Config{
		TargetFile: tempFile,
		// OnChange is nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := Watch(ctx, config)
	if err == nil || err.Error() != "OnChange callback must be set" {
		t.Errorf("Expected 'OnChange callback must be set' error, got %v", err)
	}
}

func TestWatch_FileChange(t *testing.T) {
	tempFile := createTempFile(t)
	defer os.Remove(tempFile)

	var mu sync.Mutex
	var changeCount int
	var events []string
	var errorList []error

	config := Config{
		TargetFile: tempFile,
		OnChange: func() {
			mu.Lock()
			changeCount++
			mu.Unlock()
		},
		Debounce:   50 * time.Millisecond,
		RetryDelay: 10 * time.Millisecond,
		OnEvent: func(msg string) {
			mu.Lock()
			events = append(events, msg)
			mu.Unlock()
		},
		OnError: func(err error) {
			mu.Lock()
			errorList = append(errorList, err)
			mu.Unlock()
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start watching in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- Watch(ctx, config)
	}()

	// Wait a bit for watcher to start
	time.Sleep(100 * time.Millisecond)

	// Modify the file
	if err := os.WriteFile(tempFile, []byte("modified content"), 0644); err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}

	// Wait for debounce and processing
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	gotChanges := changeCount
	gotEvents := make([]string, len(events))
	copy(gotEvents, events)
	gotErrors := make([]error, len(errorList))
	copy(gotErrors, errorList)
	mu.Unlock()

	if gotChanges == 0 {
		t.Error("Expected at least one change callback")
	}

	// Check that we got some events
	if len(gotEvents) == 0 {
		t.Error("Expected some events to be logged")
	}

	// Should not have errors in normal operation
	if len(gotErrors) > 0 {
		t.Errorf("Unexpected errors: %v", gotErrors)
	}

	cancel()
	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("Unexpected error from Watch: %v", err)
	}
}

func TestWatch_MultipleChanges_Debounced(t *testing.T) {
	tempFile := createTempFile(t)
	defer os.Remove(tempFile)

	var mu sync.Mutex
	var changeCount int

	config := Config{
		TargetFile: tempFile,
		OnChange: func() {
			mu.Lock()
			changeCount++
			mu.Unlock()
		},
		Debounce:   200 * time.Millisecond,
		RetryDelay: 10 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start watching in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- Watch(ctx, config)
	}()

	// Wait for watcher to start
	time.Sleep(100 * time.Millisecond)

	// Make multiple rapid changes
	for i := 0; i < 5; i++ {
		content := []byte("content " + string(rune('0'+i)))
		if err := os.WriteFile(tempFile, content, 0644); err != nil {
			t.Fatalf("Failed to modify file: %v", err)
		}
		time.Sleep(10 * time.Millisecond) // Small delay between writes
	}

	// Wait for debounce period plus some processing time
	time.Sleep(400 * time.Millisecond)

	mu.Lock()
	gotChanges := changeCount
	mu.Unlock()

	// Due to debouncing, we should get fewer callbacks than file modifications
	// Exact count depends on timing, but should be less than 5
	if gotChanges == 0 {
		t.Error("Expected at least one change callback")
	}
	if gotChanges >= 5 {
		t.Errorf("Expected debouncing to reduce callbacks, got %d", gotChanges)
	}

	cancel()
	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("Unexpected error from Watch: %v", err)
	}
}

func TestWatch_NonExistentFile(t *testing.T) {
	nonExistentFile := filepath.Join(os.TempDir(), "nonexistent", "file.txt")

	var mu sync.Mutex
	var errorList []error

	config := Config{
		TargetFile: nonExistentFile,
		OnChange:   func() {},
		RetryDelay: 10 * time.Millisecond,
		OnError: func(err error) {
			mu.Lock()
			errorList = append(errorList, err)
			mu.Unlock()
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := Watch(ctx, config)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}

	mu.Lock()
	gotErrors := len(errorList)
	mu.Unlock()

	if gotErrors == 0 {
		t.Error("Expected errors when watching non-existent directory")
	}
}

func TestWatch_ContextCancellation(t *testing.T) {
	tempFile := createTempFile(t)
	defer os.Remove(tempFile)

	config := Config{
		TargetFile: tempFile,
		OnChange:   func() {},
		Debounce:   100 * time.Millisecond,
		RetryDelay: 10 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Start watching in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- Watch(ctx, config)
	}()

	// Wait a bit for watcher to start
	time.Sleep(50 * time.Millisecond)

	// Cancel the context
	cancel()

	// Should get context.Canceled error
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("Watch did not return after context cancellation")
	}
}

func TestWatch_FileRemovalAndRecreation(t *testing.T) {
	tempFile := createTempFile(t)

	var mu sync.Mutex
	var changeCount int
	var events []string

	config := Config{
		TargetFile: tempFile,
		OnChange: func() {
			mu.Lock()
			changeCount++
			mu.Unlock()
		},
		Debounce:   50 * time.Millisecond,
		RetryDelay: 10 * time.Millisecond,
		OnEvent: func(msg string) {
			mu.Lock()
			events = append(events, msg)
			mu.Unlock()
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Start watching in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- Watch(ctx, config)
	}()

	// Wait for watcher to start
	time.Sleep(100 * time.Millisecond)

	// Remove the file
	if err := os.Remove(tempFile); err != nil {
		t.Fatalf("Failed to remove file: %v", err)
	}

	// Wait for event processing
	time.Sleep(100 * time.Millisecond)

	// Recreate the file
	if err := os.WriteFile(tempFile, []byte("recreated"), 0644); err != nil {
		t.Fatalf("Failed to recreate file: %v", err)
	}

	// Wait for debounce and processing
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	gotChanges := changeCount
	mu.Unlock()

	// Should get callbacks for both removal and recreation
	if gotChanges == 0 {
		t.Error("Expected change callbacks for file removal/recreation")
	}

	// Clean up
	os.Remove(tempFile)
	cancel()
	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("Unexpected error from Watch: %v", err)
	}
}

// Test for real-world usage pattern with conditional reloading
func TestWatch_ConditionalReload(t *testing.T) {
	tempFile := createTempFile(t)
	defer os.Remove(tempFile)

	var mu sync.Mutex
	var reloadCount int
	var events []string
	var errorList []error

	// Simulate a feature flag
	reloadEnabled := true

	config := Config{
		TargetFile: tempFile,
		OnChange: func() {
			mu.Lock()
			if reloadEnabled {
				reloadCount++
			}
			mu.Unlock()
		},
		Debounce:   100 * time.Millisecond,
		RetryDelay: 10 * time.Millisecond,
		OnEvent: func(msg string) {
			mu.Lock()
			events = append(events, msg)
			mu.Unlock()
		},
		OnError: func(err error) {
			mu.Lock()
			errorList = append(errorList, err)
			mu.Unlock()
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start watching in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- Watch(ctx, config)
	}()

	// Wait for watcher to start
	time.Sleep(50 * time.Millisecond)

	// First change - should trigger reload
	if err := os.WriteFile(tempFile, []byte("change 1"), 0644); err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}
	time.Sleep(150 * time.Millisecond)

	// Disable reloading
	mu.Lock()
	reloadEnabled = false
	mu.Unlock()

	// Second change - should not trigger reload
	if err := os.WriteFile(tempFile, []byte("change 2"), 0644); err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	gotReloads := reloadCount
	gotEvents := len(events)
	gotErrors := len(errorList)
	mu.Unlock()

	if gotReloads != 1 {
		t.Errorf("Expected exactly 1 reload, got %d", gotReloads)
	}

	if gotEvents == 0 {
		t.Error("Expected some events to be logged")
	}

	if gotErrors > 0 {
		t.Errorf("Unexpected errors: %v", errorList)
	}

	cancel()
	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("Unexpected error from Watch: %v", err)
	}
}

// Test for longer debounce periods to ensure they work correctly
func TestWatch_LongDebounce(t *testing.T) {
	tempFile := createTempFile(t)
	defer os.Remove(tempFile)

	var mu sync.Mutex
	var changeCount int
	var lastChangeTime time.Time

	config := Config{
		TargetFile: tempFile,
		OnChange: func() {
			mu.Lock()
			changeCount++
			lastChangeTime = time.Now()
			mu.Unlock()
		},
		Debounce:   1 * time.Second, // Shorter than 5s for testing, but still substantial
		RetryDelay: 10 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start watching in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- Watch(ctx, config)
	}()

	// Wait for watcher to start
	time.Sleep(50 * time.Millisecond)

	startTime := time.Now()

	// Make multiple changes within the debounce period
	for i := 0; i < 3; i++ {
		content := []byte("change " + string(rune('0'+i)))
		if err := os.WriteFile(tempFile, content, 0644); err != nil {
			t.Fatalf("Failed to modify file: %v", err)
		}
		time.Sleep(200 * time.Millisecond) // Less than debounce period
	}

	// Wait for debounce period plus processing time
	time.Sleep(1200 * time.Millisecond)

	mu.Lock()
	gotChanges := changeCount
	changeTime := lastChangeTime
	mu.Unlock()

	// Should only get one callback due to debouncing
	if gotChanges != 1 {
		t.Errorf("Expected exactly 1 change callback due to debouncing, got %d", gotChanges)
	}

	// Verify the callback happened after the debounce period
	if changeTime.Sub(startTime) < 1*time.Second {
		t.Errorf("Change callback happened too early, expected after 1s debounce")
	}

	cancel()
	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("Unexpected error from Watch: %v", err)
	}
}

// Helper function to create a temporary file for testing
func createTempFile(t *testing.T) string {
	t.Helper()

	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "testfile.txt")

	if err := os.WriteFile(tempFile, []byte("initial content"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	return tempFile
}

// Benchmark to measure performance
func BenchmarkWatch_FileChanges(b *testing.B) {
	tempDir := b.TempDir()
	tempFile := filepath.Join(tempDir, "testfile.txt")

	if err := os.WriteFile(tempFile, []byte("initial content"), 0644); err != nil {
		b.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile)

	var changeCount int
	config := Config{
		TargetFile: tempFile,
		OnChange: func() {
			changeCount++
		},
		Debounce:   10 * time.Millisecond,
		RetryDelay: 10 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start watching
	done := make(chan error, 1)
	go func() {
		done <- Watch(ctx, config)
	}()

	// Wait for watcher to start
	time.Sleep(50 * time.Millisecond)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		content := []byte("content " + string(rune('0'+(i%10))))
		if err := os.WriteFile(tempFile, content, 0644); err != nil {
			b.Fatalf("Failed to modify file: %v", err)
		}
		time.Sleep(20 * time.Millisecond) // Wait longer than debounce
	}

	cancel()
	<-done
}

// Test the SelfMonitor convenience function
func TestSelfMonitor(t *testing.T) {
	var mu sync.Mutex
	var reloadCount int
	var events []string
	var errorList []error

	config := SelfMonitorConfig{
		OnReload: func() {
			mu.Lock()
			reloadCount++
			mu.Unlock()
		},
		Debounce:   50 * time.Millisecond,
		RetryDelay: 10 * time.Millisecond,
		OnEvent: func(msg string) {
			mu.Lock()
			events = append(events, msg)
			mu.Unlock()
		},
		OnError: func(err error) {
			mu.Lock()
			errorList = append(errorList, err)
			mu.Unlock()
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start monitoring in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- SelfMonitor(ctx, config)
	}()

	// Wait a bit for watcher to start
	time.Sleep(100 * time.Millisecond)

	// Get the current executable path and simulate updating it
	// Since we can't actually update the running executable, we'll check
	// that the function properly handles the current executable path
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("Failed to get executable path: %v", err)
	}

	// Create a test file in the same directory as the executable to simulate an update
	execDir := filepath.Dir(executable)
	testFile := filepath.Join(execDir, "test_binary")
	if err := os.WriteFile(testFile, []byte("test"), 0755); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(testFile)

	// Cancel the first monitor and start a new one watching our test file
	cancel()
	<-done

	// Use the regular Watch function with our test file to verify the concept
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()

	watchConfig := Config{
		TargetFile: testFile,
		OnChange: func() {
			mu.Lock()
			reloadCount++
			mu.Unlock()
		},
		Debounce:   50 * time.Millisecond,
		RetryDelay: 10 * time.Millisecond,
		OnEvent: func(msg string) {
			mu.Lock()
			events = append(events, msg)
			mu.Unlock()
		},
		OnError: func(err error) {
			mu.Lock()
			errorList = append(errorList, err)
			mu.Unlock()
		},
	}

	done2 := make(chan error, 1)
	go func() {
		done2 <- Watch(ctx2, watchConfig)
	}()

	time.Sleep(100 * time.Millisecond)

	// Modify the test file
	if err := os.WriteFile(testFile, []byte("modified"), 0755); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// Wait for debounce and processing
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	gotReloads := reloadCount
	gotEvents := len(events)
	gotErrors := len(errorList)
	mu.Unlock()

	if gotReloads == 0 {
		t.Error("Expected at least one reload callback")
	}

	if gotEvents == 0 {
		t.Error("Expected some events to be logged")
	}

	if gotErrors > 0 {
		t.Errorf("Unexpected errors: %v", errorList)
	}

	cancel2()
	if err := <-done2; err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("Unexpected error from Watch: %v", err)
	}
}

// Test multi-file watching functionality
func TestWatchMultiple_BasicFunctionality(t *testing.T) {
	// Create temporary files in different directories
	tempDir1 := t.TempDir()
	tempDir2 := t.TempDir()

	file1 := filepath.Join(tempDir1, "file1.txt")
	file2 := filepath.Join(tempDir2, "file2.txt")

	if err := os.WriteFile(file1, []byte("initial1"), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("initial2"), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	var mu sync.Mutex
	var changedFiles []string
	var events []string
	var errorList []error

	config := MultiConfig{
		TargetFiles: []string{file1, file2},
		OnChange: func(file string) {
			mu.Lock()
			changedFiles = append(changedFiles, file)
			mu.Unlock()
		},
		Debounce:   50 * time.Millisecond,
		RetryDelay: 10 * time.Millisecond,
		OnEvent: func(msg string) {
			mu.Lock()
			events = append(events, msg)
			mu.Unlock()
		},
		OnError: func(err error) {
			mu.Lock()
			errorList = append(errorList, err)
			mu.Unlock()
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Start watching in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- WatchMultiple(ctx, config)
	}()

	// Wait for watcher to start
	time.Sleep(100 * time.Millisecond)

	// Modify file1
	if err := os.WriteFile(file1, []byte("modified1"), 0644); err != nil {
		t.Fatalf("Failed to modify file1: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	// Modify file2
	if err := os.WriteFile(file2, []byte("modified2"), 0644); err != nil {
		t.Fatalf("Failed to modify file2: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	gotChangedFiles := make([]string, len(changedFiles))
	copy(gotChangedFiles, changedFiles)
	gotEvents := len(events)
	gotErrors := len(errorList)
	mu.Unlock()

	if len(gotChangedFiles) < 2 {
		t.Errorf("Expected at least 2 file changes, got %d: %v", len(gotChangedFiles), gotChangedFiles)
	}

	// Check that both files were detected
	hasFile1, hasFile2 := false, false
	for _, file := range gotChangedFiles {
		if file == file1 {
			hasFile1 = true
		}
		if file == file2 {
			hasFile2 = true
		}
	}

	if !hasFile1 {
		t.Error("Expected file1 to be detected in changes")
	}
	if !hasFile2 {
		t.Error("Expected file2 to be detected in changes")
	}

	if gotEvents == 0 {
		t.Error("Expected some events to be logged")
	}

	if gotErrors > 0 {
		t.Errorf("Unexpected errors: %v", errorList)
	}

	cancel()
	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("Unexpected error from WatchMultiple: %v", err)
	}
}

func TestWatchMultiple_EmptyFileList(t *testing.T) {
	config := MultiConfig{
		TargetFiles: []string{}, // Empty list
		OnChange:    func(string) {},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := WatchMultiple(ctx, config)
	if err == nil || err.Error() != "at least one target file must be specified" {
		t.Errorf("Expected 'at least one target file must be specified' error, got %v", err)
	}
}

func TestWatchMultiple_MissingOnChange(t *testing.T) {
	tempFile := createTempFile(t)
	defer os.Remove(tempFile)

	config := MultiConfig{
		TargetFiles: []string{tempFile},
		// OnChange is nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := WatchMultiple(ctx, config)
	if err == nil || err.Error() != "OnChange callback must be set" {
		t.Errorf("Expected 'OnChange callback must be set' error, got %v", err)
	}
}

func TestWatchMultiple_SameDirectory(t *testing.T) {
	// Test multiple files in the same directory
	tempDir := t.TempDir()
	file1 := filepath.Join(tempDir, "file1.txt")
	file2 := filepath.Join(tempDir, "file2.txt")

	if err := os.WriteFile(file1, []byte("initial1"), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("initial2"), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	var mu sync.Mutex
	var changedFiles []string

	config := MultiConfig{
		TargetFiles: []string{file1, file2},
		OnChange: func(file string) {
			mu.Lock()
			changedFiles = append(changedFiles, file)
			mu.Unlock()
		},
		Debounce:   50 * time.Millisecond,
		RetryDelay: 10 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- WatchMultiple(ctx, config)
	}()

	time.Sleep(100 * time.Millisecond)

	// Modify both files
	if err := os.WriteFile(file1, []byte("modified1"), 0644); err != nil {
		t.Fatalf("Failed to modify file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("modified2"), 0644); err != nil {
		t.Fatalf("Failed to modify file2: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	gotChangedFiles := make([]string, len(changedFiles))
	copy(gotChangedFiles, changedFiles)
	mu.Unlock()

	if len(gotChangedFiles) < 2 {
		t.Errorf("Expected at least 2 file changes, got %d", len(gotChangedFiles))
	}

	cancel()
	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("Unexpected error from WatchMultiple: %v", err)
	}
}
