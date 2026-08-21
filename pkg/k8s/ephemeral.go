package k8s

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// EnsureEphemeralContainer attaches an ephemeral debug container if not already running, and waits for it to be Running
func (c *ClientContext) EnsureEphemeralContainer(
	ctx context.Context,
	podName string,
	targetContainer string,
	debugImage string,
	logWriter io.Writer,
) (string, error) {
	if debugImage == "" {
		debugImage = "nicolaka/netshoot:latest"
	}

	pod, err := c.GetPod(ctx, podName)
	if err != nil {
		return "", err
	}

	// Check if an existing kcap/knet ephemeral container is already running on this pod
	for _, status := range pod.Status.EphemeralContainerStatuses {
		if status.State.Running != nil && (len(status.Name) >= 10 && status.Name[:10] == "knet-debug") {
			return status.Name, nil
		}
	}

	// Generate unique debug container name
	randBytes := make([]byte, 3)
	_, _ = rand.Read(randBytes)
	debugContainerName := fmt.Sprintf("knet-debug-%s", hex.EncodeToString(randBytes))

	if logWriter != nil {
		fmt.Fprintf(logWriter, "==> Attaching ephemeral debug container '%s' (image: %s) to pod '%s'...\n",
			debugContainerName, debugImage, podName)
	}

	privileged := true
	ec := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:            debugContainerName,
			Image:           debugImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         []string{"sleep", "86400"},
			Stdin:           true,
			TTY:             false,
			SecurityContext: &corev1.SecurityContext{
				Privileged: &privileged,
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{"NET_ADMIN", "NET_RAW"},
				},
			},
		},
		TargetContainerName: targetContainer,
	}

	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, ec)

	_, err = c.Clientset.CoreV1().Pods(c.Namespace).UpdateEphemeralContainers(ctx, podName, pod, metav1.UpdateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to attach ephemeral container (ensure cluster supports EphemeralContainers): %w", err)
	}

	if logWriter != nil {
		fmt.Fprintf(logWriter, "==> Waiting for debug container '%s' to enter Running state...\n", debugContainerName)
	}

	err = wait.PollUntilContextTimeout(ctx, 1*time.Second, 60*time.Second, true, func(ctx context.Context) (bool, error) {
		currentPod, getErr := c.Clientset.CoreV1().Pods(c.Namespace).Get(ctx, podName, metav1.GetOptions{})
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

// RunInEphemeral executes a command inside an ephemeral container attached to the target pod
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
	debugContainerName, err := c.EnsureEphemeralContainer(ctx, podName, targetContainer, debugImage, logWriter)
	if err != nil {
		return err
	}

	return c.ExecCommand(ctx, ExecOptions{
		Namespace:     c.Namespace,
		PodName:       podName,
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
