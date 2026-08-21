package output

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
)

// FindWireshark searches for the Wireshark binary in standard OS paths or PATH
func FindWireshark(customPath string) (string, error) {
	if customPath != "" {
		if _, err := os.Stat(customPath); err == nil {
			return customPath, nil
		}
		return "", fmt.Errorf("specified Wireshark path '%s' not found", customPath)
	}

	// 1. Check if 'wireshark' is in $PATH
	if path, err := exec.LookPath("wireshark"); err == nil {
		return path, nil
	}

	// 2. Check OS-specific default installation paths
	var candidates []string
	switch runtime.GOOS {
	case "darwin": // macOS
		candidates = []string{
			"/Applications/Wireshark.app/Contents/MacOS/Wireshark",
			"/Applications/Wireshark.app/Contents/MacOS/wireshark",
			"/Applications/Wireshark.app/Contents/Resources/bin/wireshark",
			"/opt/homebrew/bin/wireshark",
			"/usr/local/bin/wireshark",
		}
		if u, err := user.Current(); err == nil && u.HomeDir != "" {
			candidates = append(candidates, filepath.Join(u.HomeDir, "Applications/Wireshark.app/Contents/MacOS/Wireshark"))
			candidates = append(candidates, filepath.Join(u.HomeDir, "Applications/Wireshark.app/Contents/MacOS/wireshark"))
		}
	case "linux":
		candidates = []string{
			"/usr/bin/wireshark",
			"/usr/local/bin/wireshark",
			"/snap/bin/wireshark",
			"/usr/bin/tshark",
		}
	case "windows":
		candidates = []string{
			`C:\Program Files\Wireshark\Wireshark.exe`,
			`C:\Program Files (x86)\Wireshark\Wireshark.exe`,
		}
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}

	return "", fmt.Errorf("wireshark binary not found. Please install Wireshark or specify path with --wireshark-path (or use -o to save to file)")
}

// WiresharkProcess encapsulates the running Wireshark instance, its stdin pipe, and exit notification
type WiresharkProcess struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	exitChan chan struct{}
	once     sync.Once
}

// StartWireshark spawns Wireshark in live capture mode reading from stdin
func StartWireshark(wiresharkPath string) (*WiresharkProcess, error) {
	// -k : start capturing immediately
	// -i - : capture from standard input
	cmd := exec.Command(wiresharkPath, "-k", "-i", "-")
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Run Wireshark in its own process group
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe for Wireshark: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start Wireshark: %w", err)
	}

	wp := &WiresharkProcess{
		cmd:      cmd,
		stdin:    stdin,
		exitChan: make(chan struct{}),
	}

	// Monitor Wireshark process termination in background
	go func() {
		_ = cmd.Wait()
		wp.once.Do(func() {
			close(wp.exitChan)
		})
	}()

	return wp, nil
}

// ExitChan returns a channel that is closed when the Wireshark GUI process terminates
func (w *WiresharkProcess) ExitChan() <-chan struct{} {
	return w.exitChan
}

// Write writes bytes into Wireshark's stdin
func (w *WiresharkProcess) Write(p []byte) (n int, err error) {
	select {
	case <-w.exitChan:
		return 0, io.EOF
	default:
	}
	return w.stdin.Write(p)
}

// Close closes the stdin pipe
func (w *WiresharkProcess) Close() error {
	w.once.Do(func() {
		close(w.exitChan)
	})
	if w.stdin != nil {
		_ = w.stdin.Close()
	}
	return nil
}
