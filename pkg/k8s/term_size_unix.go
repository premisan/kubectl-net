//go:build !windows

package k8s

import (
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"
	"k8s.io/client-go/tools/remotecommand"
)

func (q *terminalSizeQueue) startMonitoring() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGWINCH)

	go func() {
		defer signal.Stop(sigChan)

		var lastWidth, lastHeight uint16
		if w, h, err := term.GetSize(q.fd); err == nil && w > 0 && h > 0 {
			lastWidth = uint16(w)
			lastHeight = uint16(h)
		}

		for {
			select {
			case <-q.stop:
				return
			case <-sigChan:
				w, h, err := term.GetSize(q.fd)
				if err != nil || w <= 0 || h <= 0 {
					continue
				}

				newWidth := uint16(w)
				newHeight := uint16(h)

				if newWidth == lastWidth && newHeight == lastHeight {
					continue
				}
				lastWidth = newWidth
				lastHeight = newHeight

				size := remotecommand.TerminalSize{
					Width:  newWidth,
					Height: newHeight,
				}

				select {
				case q.resize <- size:
				case <-q.stop:
					return
				default:
					select {
					case <-q.resize:
					default:
					}
					select {
					case q.resize <- size:
					default:
					}
				}
			}
		}
	}()
}
