package k8s

import (
	"context"
	"fmt"
	"io"
)

// DirectCaptureEngine handles packet capture directly on the existing container via exec
type DirectCaptureEngine struct {
	clientCtx       *ClientContext
	PodName         string
	TargetContainer string
	Interface       string
	Filter          string
	Verbose         bool
}

// NewDirectCaptureEngine creates an instance of DirectCaptureEngine
func NewDirectCaptureEngine(
	clientCtx *ClientContext,
	podName string,
	targetContainer string,
	iface string,
	filter string,
	verbose bool,
) *DirectCaptureEngine {
	if iface == "" {
		iface = "any"
	}
	return &DirectCaptureEngine{
		clientCtx:       clientCtx,
		PodName:         podName,
		TargetContainer: targetContainer,
		Interface:       iface,
		Filter:          filter,
		Verbose:         verbose,
	}
}

// Start executes tcpdump directly inside the target container and streams output
func (d *DirectCaptureEngine) Start(ctx context.Context, pcapWriter io.Writer, logWriter io.Writer) error {
	// 1. Verify Pod is running
	_, err := d.clientCtx.GetPod(ctx, d.PodName)
	if err != nil {
		return err
	}

	if logWriter != nil {
		fmt.Fprintf(logWriter, "==> Executing tcpdump directly in container '%s' of pod '%s'...\n",
			d.TargetContainer, d.PodName)
		fmt.Fprintf(logWriter, "==> Interface: '%s'\n", d.Interface)
		if d.Filter != "" {
			fmt.Fprintf(logWriter, "==> Packet filter: %s\n", d.Filter)
		}
	}

	// 2. Construct tcpdump command
	tcpdumpCmd := []string{"tcpdump", "-i", d.Interface, "-U", "-w", "-"}
	if d.Filter != "" {
		tcpdumpCmd = append(tcpdumpCmd, d.Filter)
	}

	// 3. Execute and stream
	return d.clientCtx.ExecCommand(ctx, ExecOptions{
		Namespace:     d.clientCtx.Namespace,
		PodName:       d.PodName,
		ContainerName: d.TargetContainer,
		Command:       tcpdumpCmd,
		Stdin:         nil,
		Stdout:        pcapWriter,
		Stderr:        logWriter,
		TTY:           false,
	})
}
