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

// EphemeralCaptureEngine manages packet capture via Kubernetes Ephemeral Containers (kubectl debug mechanism)
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

// Start attaches an ephemeral debug container to the target pod and streams tcpdump output
func (e *EphemeralCaptureEngine) Start(ctx context.Context, pcapWriter io.Writer, logWriter io.Writer) error {
	clientset := e.clientCtx.Clientset
	ns := e.clientCtx.Namespace

	// 1. Fetch current Pod
	pod, err := clientset.CoreV1().Pods(ns).Get(ctx, e.PodName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod '%s/%s': %w", ns, e.PodName, err)
	}

	if pod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf("target pod '%s' is not in Running state (current phase: %s)", e.PodName, pod.Status.Phase)
	}

	// 2. Generate unique ephemeral container name
	randBytes := make([]byte, 3)
	_, _ = rand.Read(randBytes)
	debugContainerName := fmt.Sprintf("kcap-debug-%s", hex.EncodeToString(randBytes))

	if logWriter != nil {
		fmt.Fprintf(logWriter, "==> Attaching ephemeral debug container '%s' (image: %s) to pod '%s'...\n",
			debugContainerName, e.DebugImage, e.PodName)
	}

	// 3. Construct Ephemeral Container definition
	privileged := true
	ec := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:            debugContainerName,
			Image:           e.DebugImage,
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
		TargetContainerName: e.TargetContainer,
	}

	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, ec)

	// 4. Update EphemeralContainers subresource
	_, err = clientset.CoreV1().Pods(ns).UpdateEphemeralContainers(ctx, e.PodName, pod, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to attach ephemeral container (ensure cluster supports EphemeralContainers): %w", err)
	}

	// 5. Wait for the ephemeral container to be Running
	if logWriter != nil {
		fmt.Fprintf(logWriter, "==> Waiting for debug container '%s' to enter Running state...\n", debugContainerName)
	}

	err = wait.PollUntilContextTimeout(ctx, 1*time.Second, 60*time.Second, true, func(ctx context.Context) (bool, error) {
		currentPod, getErr := clientset.CoreV1().Pods(ns).Get(ctx, e.PodName, metav1.GetOptions{})
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
		return fmt.Errorf("timeout or error waiting for ephemeral container: %w", err)
	}

	if logWriter != nil {
		fmt.Fprintf(logWriter, "==> Debug container is running. Starting packet capture on interface '%s'...\n", e.Interface)
		if e.Filter != "" {
			fmt.Fprintf(logWriter, "==> Packet filter: %s\n", e.Filter)
		}
	}

	// 6. Build tcpdump command
	// -i <iface>: network interface
	// -U: packet-buffered (immediate flush)
	// -w -: write raw pcap stream to stdout
	tcpdumpCmd := []string{"tcpdump", "-i", e.Interface, "-U", "-w", "-"}
	if e.Filter != "" {
		tcpdumpCmd = append(tcpdumpCmd, e.Filter)
	}

	// 7. Execute tcpdump and stream stdout to pcapWriter
	return StreamExec(
		ctx,
		clientset,
		e.clientCtx.Config,
		ns,
		e.PodName,
		debugContainerName,
		tcpdumpCmd,
		nil,
		pcapWriter,
		logWriter, // stderr for tcpdump statistics & logs
	)
}
