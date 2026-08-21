package k8s

import (
	"context"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// ExecOptions holds parameters for remote command execution
type ExecOptions struct {
	Namespace     string
	PodName       string
	ContainerName string
	Command       []string
	Stdin         io.Reader
	Stdout        io.Writer
	Stderr        io.Writer
	TTY           bool
}

// ExecCommand executes a command inside a target container
func (c *ClientContext) ExecCommand(ctx context.Context, opts ExecOptions) error {
	req := c.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(opts.PodName).
		Namespace(opts.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: opts.ContainerName,
			Command:   opts.Command,
			Stdin:     opts.Stdin != nil,
			Stdout:    opts.Stdout != nil,
			Stderr:    opts.Stderr != nil,
			TTY:       opts.TTY,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(c.Config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create SPDY executor: %w", err)
	}

	if opts.TTY {
		// Set terminal to raw mode if stdin is a terminal
		if file, ok := opts.Stdin.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
			oldState, err := term.MakeRaw(int(file.Fd()))
			if err == nil {
				defer func() {
					_ = term.Restore(int(file.Fd()), oldState)
				}()
			}
		}
	}

	streamOpts := remotecommand.StreamOptions{
		Stdin:  opts.Stdin,
		Stdout: opts.Stdout,
		Stderr: opts.Stderr,
		Tty:    opts.TTY,
	}

	err = executor.StreamWithContext(ctx, streamOpts)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}

	return nil
}
