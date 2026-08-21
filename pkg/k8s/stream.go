package k8s

import (
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// StreamExec executes a command inside a target container and streams stdout/stderr
func StreamExec(
	ctx context.Context,
	clientset kubernetes.Interface,
	config *rest.Config,
	namespace string,
	podName string,
	containerName string,
	cmd []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) error {
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   cmd,
			Stdin:     stdin != nil,
			Stdout:    stdout != nil,
			Stderr:    stderr != nil,
			TTY:       false,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create SPDY executor: %w", err)
	}

	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    false,
	})

	if err != nil {
		// Context cancellation is normal on exit
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("remote execution failed: %w", err)
	}

	return nil
}
