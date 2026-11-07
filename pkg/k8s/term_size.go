package k8s

import (
	"sync"

	"golang.org/x/term"
	"k8s.io/client-go/tools/remotecommand"
)

type terminalSizeQueue struct {
	fd     int
	resize chan remotecommand.TerminalSize
	stop   chan struct{}
	once   sync.Once
}

func newTerminalSizeQueue(fd int) *terminalSizeQueue {
	q := &terminalSizeQueue{
		fd:     fd,
		resize: make(chan remotecommand.TerminalSize, 2),
		stop:   make(chan struct{}),
	}

	// Enqueue initial size immediately so the remote PTY is initialized with matching dimensions
	if w, h, err := term.GetSize(fd); err == nil && w > 0 && h > 0 {
		q.resize <- remotecommand.TerminalSize{
			Width:  uint16(w),
			Height: uint16(h),
		}
	}

	q.startMonitoring()
	return q
}

// Next returns the new terminal size after the terminal has been resized.
// It returns nil when monitoring has been stopped.
func (q *terminalSizeQueue) Next() *remotecommand.TerminalSize {
	select {
	case <-q.stop:
		return nil
	default:
	}

	select {
	case size, ok := <-q.resize:
		if !ok {
			return nil
		}
		return &size
	case <-q.stop:
		return nil
	}
}

// Stop terminates monitoring and causes Next() to return nil.
func (q *terminalSizeQueue) Stop() {
	q.once.Do(func() {
		close(q.stop)
	})
}
