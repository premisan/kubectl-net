package k8s

import (
	"testing"
	"time"

	"k8s.io/client-go/tools/remotecommand"
)

func TestTerminalSizeQueue_Basic(t *testing.T) {
	q := &terminalSizeQueue{
		fd:     -1,
		resize: make(chan remotecommand.TerminalSize, 2),
		stop:   make(chan struct{}),
	}

	// Verify Stop returns nil on Next()
	q.Stop()
	q.Stop() // Verify idempotent stop

	if size := q.Next(); size != nil {
		t.Fatalf("expected nil from Next() after Stop(), got %+v", size)
	}
}

func TestTerminalSizeQueue_QueueAndStop(t *testing.T) {
	q := &terminalSizeQueue{
		fd:     -1,
		resize: make(chan remotecommand.TerminalSize, 2),
		stop:   make(chan struct{}),
	}

	expected := remotecommand.TerminalSize{Width: 120, Height: 40}
	q.resize <- expected

	size := q.Next()
	if size == nil {
		t.Fatalf("expected non-nil TerminalSize, got nil")
	}
	if size.Width != 120 || size.Height != 40 {
		t.Fatalf("expected size 120x40, got %dx%d", size.Width, size.Height)
	}

	// Stop while waiting
	go func() {
		time.Sleep(20 * time.Millisecond)
		q.Stop()
	}()

	sizeAfterStop := q.Next()
	if sizeAfterStop != nil {
		t.Fatalf("expected nil after stop, got %+v", sizeAfterStop)
	}
}

func TestNewTerminalSizeQueue_NonTerminalFd(t *testing.T) {
	// fd -1 is not a terminal, should not fail or panic
	q := newTerminalSizeQueue(-1)
	defer q.Stop()

	// Should not block indefinitely when stopped
	done := make(chan struct{})
	go func() {
		q.Stop()
		_ = q.Next()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for terminal size queue to stop")
	}
}
