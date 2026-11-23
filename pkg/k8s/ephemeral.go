package k8s

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	// DefaultDebugImage is the default container image used for network debugging subcommands.
	DefaultDebugImage = "nicolaka/netshoot:latest"
	// DefaultEphemeralWaitTimeout is the default timeout to wait for an ephemeral container to enter Running state.
	DefaultEphemeralWaitTimeout = 180 * time.Second
)

// EphemeralOptions configures how an ephemeral debug container is attached or reused.
type EphemeralOptions struct {
	PodName         string
	TargetContainer string
	DebugImage      string
	NamePrefix      string
	Privileged      bool
	WaitTimeout     time.Duration
	LogWriter       io.Writer
}

func findReusableEphemeralContainer(pod *corev1.Pod, debugImage string, requirePrivileged bool) string {
	specByName := make(map[string]corev1.EphemeralContainer, len(pod.Spec.EphemeralContainers))
	for _, ec := range pod.Spec.EphemeralContainers {
		specByName[ec.Name] = ec
	}

	for _, status := range pod.Status.EphemeralContainerStatuses {
		if status.State.Running == nil {
			continue
		}
		if !strings.HasPrefix(status.Name, "kubectl-net-") && !strings.HasPrefix(status.Name, "knet-debug") {
			continue
		}
		ec, ok := specByName[status.Name]
		if !ok || ec.Image != debugImage {
			continue
		}
		if requirePrivileged {
			if ec.SecurityContext == nil || ec.SecurityContext.Privileged == nil || !*ec.SecurityContext.Privileged {
				continue
			}
		}
		return status.Name
	}

	return ""
}

// EnsureEphemeralContainer attaches a privileged network debug ephemeral container if not already running,
// and waits for it to be Running.
func (c *ClientContext) EnsureEphemeralContainer(
	ctx context.Context,
	podName string,
	targetContainer string,
	debugImage string,
	logWriter io.Writer,
) (string, error) {
	return c.EnsureEphemeralContainerWithOptions(ctx, EphemeralOptions{
		PodName:         podName,
		TargetContainer: targetContainer,
		DebugImage:      debugImage,
		NamePrefix:      "kubectl-net-debug",
		Privileged:      true,
		WaitTimeout:     DefaultEphemeralWaitTimeout,
		LogWriter:       logWriter,
	})
}

// EnsureEphemeralContainerWithOptions attaches an ephemeral debug container matching opts if not already running,
// and waits for it to enter Running state.
func (c *ClientContext) EnsureEphemeralContainerWithOptions(
	ctx context.Context,
	opts EphemeralOptions,
) (string, error) {
	debugImage := opts.DebugImage
	if debugImage == "" {
		debugImage = DefaultDebugImage
	}
	namePrefix := opts.NamePrefix
	if namePrefix == "" {
		namePrefix = "kubectl-net-debug"
	}
	waitTimeout := opts.WaitTimeout
	if waitTimeout <= 0 {
		waitTimeout = DefaultEphemeralWaitTimeout
	}

	pod, err := c.GetPod(ctx, opts.PodName)
	if err != nil {
		return "", err
	}

	// Reuse an existing running kubectl-net ephemeral container only when its image (and privilege requirement) matches.
	if existing := findReusableEphemeralContainer(pod, debugImage, opts.Privileged); existing != "" {
		return existing, nil
	}

	// Generate unique debug container name
	randBytes := make([]byte, 3)
	_, _ = rand.Read(randBytes)
	debugContainerName := fmt.Sprintf("%s-%s", namePrefix, hex.EncodeToString(randBytes))

	if opts.LogWriter != nil {
		fmt.Fprintf(opts.LogWriter, "==> Attaching ephemeral debug container '%s' (image: %s) to pod '%s'...\n",
			debugContainerName, debugImage, opts.PodName)
	}

	var securityContext *corev1.SecurityContext
	if opts.Privileged {
		privileged := true
		securityContext = &corev1.SecurityContext{
			Privileged: &privileged,
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{"NET_ADMIN", "NET_RAW"},
			},
		}
	}

	ec := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:            debugContainerName,
			Image:           debugImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         []string{"sleep", "86400"},
			Stdin:           true,
			TTY:             false,
			SecurityContext: securityContext,
		},
		TargetContainerName: opts.TargetContainer,
	}

	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, ec)

	_, err = c.Clientset.CoreV1().Pods(c.Namespace).UpdateEphemeralContainers(ctx, opts.PodName, pod, metav1.UpdateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to attach ephemeral container (ensure cluster supports EphemeralContainers): %w", err)
	}

	if opts.LogWriter != nil {
		fmt.Fprintf(opts.LogWriter, "==> Waiting for debug container '%s' to enter Running state...\n", debugContainerName)
	}

	err = wait.PollUntilContextTimeout(ctx, 1*time.Second, waitTimeout, true, func(ctx context.Context) (bool, error) {
		currentPod, getErr := c.Clientset.CoreV1().Pods(c.Namespace).Get(ctx, opts.PodName, metav1.GetOptions{})
		if getErr != nil {
			return false, getErr
		}

		for _, status := range currentPod.Status.EphemeralContainerStatuses {
			if status.Name == debugContainerName {
				if status.State.Running != nil {
					return true, nil
				}
				if status.State.Waiting != nil {
					if status.State.Waiting.Reason == "ErrImagePull" || status.State.Waiting.Reason == "ImagePullBackOff" {
						return false, fmt.Errorf("image pull failed: %s", status.State.Waiting.Message)
					}
				}
				if status.State.Terminated != nil {
					return false, fmt.Errorf("debug container terminated unexpectedly: %s", status.State.Terminated.Message)
				}
			}
		}
		return false, nil
	})

	if err != nil {
		return "", fmt.Errorf("timeout or error waiting for ephemeral container: %w", err)
	}

	return debugContainerName, nil
}

// RunInEphemeral executes a command inside a privileged ephemeral debug container attached to the target pod.
func (c *ClientContext) RunInEphemeral(
	ctx context.Context,
	podName string,
	targetContainer string,
	debugImage string,
	cmd []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	tty bool,
	logWriter io.Writer,
) error {
	return c.RunInEphemeralWithOptions(
		ctx,
		EphemeralOptions{
			PodName:         podName,
			TargetContainer: targetContainer,
			DebugImage:      debugImage,
			NamePrefix:      "kubectl-net-debug",
			Privileged:      true,
			WaitTimeout:     DefaultEphemeralWaitTimeout,
			LogWriter:       logWriter,
		},
		cmd,
		stdin,
		stdout,
		stderr,
		tty,
	)
}

// RunInEphemeralWithOptions executes a command inside an ephemeral container configured by opts.
func (c *ClientContext) RunInEphemeralWithOptions(
	ctx context.Context,
	opts EphemeralOptions,
	cmd []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	tty bool,
) error {
	debugContainerName, err := c.EnsureEphemeralContainerWithOptions(ctx, opts)
	if err != nil {
		return err
	}

	return c.ExecCommand(ctx, ExecOptions{
		Namespace:     c.Namespace,
		PodName:       opts.PodName,
		ContainerName: debugContainerName,
		Command:       cmd,
		Stdin:         stdin,
		Stdout:        stdout,
		Stderr:        stderr,
		TTY:           tty,
	})
}

// EphemeralCaptureEngine manages packet capture via Kubernetes Ephemeral Containers
type EphemeralCaptureEngine struct {
	clientCtx       *ClientContext
	PodName         string
	TargetContainer string
	Interface       string
	Filter          string
	DebugImage      string
	Verbose         bool
}

// NewEphemeralCaptureEngine creates an instance of EphemeralCaptureEngine
func NewEphemeralCaptureEngine(
	clientCtx *ClientContext,
	podName string,
	targetContainer string,
	iface string,
	filter string,
	debugImage string,
	verbose bool,
) *EphemeralCaptureEngine {
	if debugImage == "" {
		debugImage = "nicolaka/netshoot:latest"
	}
	if iface == "" {
		iface = "any"
	}
	return &EphemeralCaptureEngine{
		clientCtx:       clientCtx,
		PodName:         podName,
		TargetContainer: targetContainer,
		Interface:       iface,
		Filter:          filter,
		DebugImage:      debugImage,
		Verbose:         verbose,
	}
}

// Start attaches an ephemeral debug container and streams tcpdump output
func (e *EphemeralCaptureEngine) Start(ctx context.Context, pcapWriter io.Writer, logWriter io.Writer) error {
	debugContainerName, err := e.clientCtx.EnsureEphemeralContainer(ctx, e.PodName, e.TargetContainer, e.DebugImage, logWriter)
	if err != nil {
		return err
	}

	if logWriter != nil {
		fmt.Fprintf(logWriter, "==> Debug container '%s' is ready. Starting packet capture on interface '%s'...\n", debugContainerName, e.Interface)
		if e.Filter != "" {
			fmt.Fprintf(logWriter, "==> Packet filter: %s\n", e.Filter)
		}
	}

	tcpdumpCmd := []string{"tcpdump", "-i", e.Interface, "-U", "-w", "-"}
	if e.Filter != "" {
		tcpdumpCmd = append(tcpdumpCmd, e.Filter)
	}

	return e.clientCtx.ExecCommand(ctx, ExecOptions{
		Namespace:     e.clientCtx.Namespace,
		PodName:       e.PodName,
		ContainerName: debugContainerName,
		Command:       tcpdumpCmd,
		Stdin:         nil,
		Stdout:        pcapWriter,
		Stderr:        logWriter,
		TTY:           false,
	})
}
