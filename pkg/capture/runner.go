package capture

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/premisan/kubectl-net/pkg/k8s"
	"github.com/premisan/kubectl-net/pkg/output"
)

// Runner coordinates the entire capture session
type Runner struct {
	Options *Options
}

// NewRunner creates a new Runner instance
func NewRunner(opts *Options) *Runner {
	return &Runner{
		Options: opts,
	}
}

// Run executes the packet capture workflow
func (r *Runner) Run(parentCtx context.Context) error {
	opts := r.Options

	// Log stream: When outputting pcap to stdout, logs must go to stderr
	var logWriter io.Writer = os.Stderr

	// Setup signal cancellation (SIGINT/SIGTERM)
	ctx, cancel := signal.NotifyContext(parentCtx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Fprintf(logWriter, "==> Initializing Kubernetes client...\n")

	// 1. Initialize K8s Client Context
	clientCtx, err := k8s.NewClientContext(opts.Kubeconfig, opts.KubeContext, opts.Namespace)
	if err != nil {
		return fmt.Errorf("failed to configure kubernetes client: %w", err)
	}

	ns := clientCtx.Namespace
	fmt.Fprintf(logWriter, "==> Target: Pod '%s' in namespace '%s'\n", opts.PodName, ns)

	// 2. Fetch Pod to verify and determine container
	pod, err := clientCtx.GetPod(ctx, opts.PodName)
	if err != nil {
		return err
	}

	targetContainer, err := k8s.FindTargetContainer(pod, opts.ContainerName)
	if err != nil {
		return err
	}
	fmt.Fprintf(logWriter, "==> Target container: '%s'\n", targetContainer)

	// 3. Pre-check Wireshark path early if Wireshark GUI is requested
	if opts.OutputFile == "" {
		_, err := output.FindWireshark(opts.WiresharkPath)
		if err != nil {
			return err
		}
	}

	// 4. Ensure Ephemeral Debug Container is ready BEFORE launching Wireshark
	var (
		execContainer string = targetContainer
		useEphemeral  bool   = (opts.Mode == ModeEphemeral || opts.Mode == ModeAuto)
	)

	if useEphemeral {
		debugContainerName, err := clientCtx.EnsureEphemeralContainer(ctx, opts.PodName, targetContainer, opts.DebugImage, logWriter)
		if err != nil {
			if opts.Mode == ModeEphemeral {
				return err
			}
			fmt.Fprintf(logWriter, "==> Ephemeral container mode failed (%v). Attempting direct exec fallback...\n", err)
			useEphemeral = false
		} else {
			execContainer = debugContainerName
		}
	}

	// 5. Setup Output Sink (Launch Wireshark or open file/stdout now that container is ready)
	outHandler, err := output.SetupOutput(opts.OutputFile, opts.WiresharkPath)
	if err != nil {
		return fmt.Errorf("failed to initialize output: %w", err)
	}
	defer outHandler.Close()

	fmt.Fprintf(logWriter, "==> %s\n", outHandler.Summary)
	fmt.Fprintf(logWriter, "==> Press Ctrl+C or close Wireshark to stop capture\n")

	// Automatically stop capture and exit when Wireshark GUI is closed
	if outHandler.ExitChan != nil {
		go func() {
			select {
			case <-outHandler.ExitChan:
				fmt.Fprintf(logWriter, "\n==> Wireshark was closed. Stopping packet capture...\n")
				cancel()
			case <-ctx.Done():
			}
		}()
	}

	// 6. Build tcpdump command and execute
	tcpdumpCmd := []string{"tcpdump", "-i", opts.Interface, "-U", "-w", "-"}
	if opts.Filter != "" {
		tcpdumpCmd = append(tcpdumpCmd, opts.Filter)
	}

	captureErr := clientCtx.ExecCommand(ctx, k8s.ExecOptions{
		Namespace:     clientCtx.Namespace,
		PodName:       opts.PodName,
		ContainerName: execContainer,
		Command:       tcpdumpCmd,
		Stdin:         nil,
		Stdout:        outHandler.Writer,
		Stderr:        logWriter,
		TTY:           false,
	})

	// Graceful shutdown
	if ctx.Err() != nil {
		fmt.Fprintf(logWriter, "==> Capture stopped.\n")
		return nil
	}

	return captureErr
}
