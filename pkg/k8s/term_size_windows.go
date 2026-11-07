//go:build windows

package k8s

func (q *terminalSizeQueue) startMonitoring() {
	// On Windows, SIGWINCH signal is not supported.
	// Initial terminal size was already enqueued during initialization.
}
