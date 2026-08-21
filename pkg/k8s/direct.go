package k8s

import (
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	clientset := d.clientCtx.Clientset
	ns := d.clientCtx.Namespace

	// 1. Verify Pod is running
	pod, err := clientset.CoreV1().Pods(ns).Get(ctx, d.PodName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod '%s/%s': %w", ns, d.PodName, err)
	}

	if pod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf("target pod '%s' is not in Running state (current phase: %s)", d.PodName, pod.Status.Phase)
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
	return StreamExec(
		ctx,
		clientset,
		d.clientCtx.Config,
		ns,
		d.PodName,
		d.TargetContainer,
		tcpdumpCmd,
		nil,
		pcapWriter,
		logWriter,
	)
}
