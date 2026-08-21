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

	// 3. Setup Output Sink (Wireshark / File / Stdout)
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

	// 4. Execute based on Mode
	var captureErr error
	switch opts.Mode {
	case ModeEphemeral:
		engine := k8s.NewEphemeralCaptureEngine(
			clientCtx,
			opts.PodName,
			targetContainer,
			opts.Interface,
			opts.Filter,
			opts.DebugImage,
			opts.Verbose,
		)
		captureErr = engine.Start(ctx, outHandler.Writer, logWriter)

	case ModeDirect:
		engine := k8s.NewDirectCaptureEngine(
			clientCtx,
			opts.PodName,
			targetContainer,
			opts.Interface,
			opts.Filter,
			opts.Verbose,
		)
		captureErr = engine.Start(ctx, outHandler.Writer, logWriter)

	case ModeAuto:
		// Default: Use Ephemeral Container for robust containerd/GKE/Distroless support
		engine := k8s.NewEphemeralCaptureEngine(
			clientCtx,
			opts.PodName,
			targetContainer,
			opts.Interface,
			opts.Filter,
			opts.DebugImage,
			opts.Verbose,
		)
		captureErr = engine.Start(ctx, outHandler.Writer, logWriter)
		if captureErr != nil && ctx.Err() == nil {
			// If Ephemeral Containers are unsupported on an older cluster, fallback to direct mode
			fmt.Fprintf(logWriter, "==> Ephemeral container mode failed (%v). Attempting direct exec fallback...\n", captureErr)
			directEngine := k8s.NewDirectCaptureEngine(
				clientCtx,
				opts.PodName,
				targetContainer,
				opts.Interface,
				opts.Filter,
				opts.Verbose,
			)
			captureErr = directEngine.Start(ctx, outHandler.Writer, logWriter)
		}

	default:
		return fmt.Errorf("unknown mode: %s", opts.Mode)
	}

	// Graceful shutdown
	if ctx.Err() != nil {
		fmt.Fprintf(logWriter, "==> Capture stopped.\n")
		return nil
	}

	return captureErr
}
