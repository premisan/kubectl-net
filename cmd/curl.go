package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/premisan/kubectl-net/pkg/k8s"
)

var curlCmd = &cobra.Command{
	Use:   "curl [POD_NAME] [CURL_OPTIONS...] [URL]",
	Short: "Execute curl HTTP/HTTPS requests from inside a target pod",
	Long: `Execute curl HTTP/HTTPS network tests from inside a Kubernetes pod.
Uses an Ephemeral Debug Container to test connectivity to other pods, internal services,
or external URLs directly from the target pod's network namespace.

Supports all standard curl flags (-v, -k, -X POST, -H, -d, -I, -L, --connect-timeout, etc.).`,
	Example: `  # Basic GET request from pod to internal cluster service:
  kubectl net curl my-pod http://backend-service.default.svc:8080/healthz

  # Verbose output with custom headers and method:
  kubectl net curl my-pod -v -X POST -H "Content-Type: application/json" \
    -d '{"key":"value"}' http://backend-service:8080/api

  # Insecure HTTPS request with timeout:
  kubectl net curl my-pod -k -v --connect-timeout 5 https://10.96.0.1:443

  # Target a specific namespace and container:
  kubectl net curl my-pod -n production -c app-container -I https://google.com`,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, rawArgs []string) error {
		// Handle help flags
		for _, arg := range rawArgs {
			if arg == "--help" || arg == "-h" {
				return cmd.Help()
			}
		}

		if len(rawArgs) == 0 {
			return cmd.Help()
		}

		// Parse arguments and flags manually for complete curl flag passthrough
		var (
			podName       string
			namespace     = globalNamespace
			containerName = globalContainer
			debugImage    = globalDebugImage
			kubeconfig    = globalKubeconfig
			kubeContext   = globalContext
			curlArgs      []string
		)

		i := 0
		for i < len(rawArgs) {
			arg := rawArgs[i]

			// Global / K8s flag overrides
			if arg == "-n" && i+1 < len(rawArgs) {
				namespace = rawArgs[i+1]
				i += 2
				continue
			} else if strings.HasPrefix(arg, "--namespace=") {
				namespace = strings.TrimPrefix(arg, "--namespace=")
				i++
				continue
			} else if arg == "--namespace" && i+1 < len(rawArgs) {
				namespace = rawArgs[i+1]
				i += 2
				continue
			} else if arg == "-c" && i+1 < len(rawArgs) {
				// Note: if podName is already set, -c could be curl's cookie jar (-c file).
				// If podName is NOT set yet, -c refers to target container
				if podName == "" {
					containerName = rawArgs[i+1]
					i += 2
					continue
				}
			} else if strings.HasPrefix(arg, "--container=") {
				containerName = strings.TrimPrefix(arg, "--container=")
				i++
				continue
			} else if arg == "--container" && i+1 < len(rawArgs) {
				containerName = rawArgs[i+1]
				i += 2
				continue
			} else if strings.HasPrefix(arg, "--image=") {
				debugImage = strings.TrimPrefix(arg, "--image=")
				i++
				continue
			} else if arg == "--image" && i+1 < len(rawArgs) {
				debugImage = rawArgs[i+1]
				i += 2
				continue
			} else if strings.HasPrefix(arg, "--kubeconfig=") {
				kubeconfig = strings.TrimPrefix(arg, "--kubeconfig=")
				i++
				continue
			} else if strings.HasPrefix(arg, "--context=") {
				kubeContext = strings.TrimPrefix(arg, "--context=")
				i++
				continue
			}

			// First non-flag argument is target Pod Name
			if podName == "" && !strings.HasPrefix(arg, "-") {
				podName = arg
				i++
				continue
			}

			// All other args belong to curl
			curlArgs = append(curlArgs, arg)
			i++
		}

		if podName == "" {
			return fmt.Errorf("target pod name is required. Usage: kubectl net curl [POD_NAME] [CURL_OPTIONS...] [URL]")
		}

		if len(curlArgs) == 0 {
			return fmt.Errorf("URL or curl arguments required. Example: kubectl net curl %s http://example.com", podName)
		}

		ctx := context.Background()
		clientCtx, err := k8s.NewClientContext(kubeconfig, kubeContext, namespace)
		if err != nil {
			return fmt.Errorf("failed to configure kubernetes client: %w", err)
		}

		// Verify Pod & find container
		pod, err := clientCtx.GetPod(ctx, podName)
		if err != nil {
			return err
		}

		resolvedContainer, err := k8s.FindTargetContainer(pod, containerName)
		if err != nil {
			return err
		}

		finalCurlCmd := append([]string{"curl"}, curlArgs...)
		fmt.Fprintf(os.Stderr, "==> Executing in pod '%s' (namespace: %s, container: %s): %s\n",
			podName, clientCtx.Namespace, resolvedContainer, strings.Join(finalCurlCmd, " "))

		return clientCtx.RunInEphemeral(
			ctx,
			podName,
			resolvedContainer,
			debugImage,
			finalCurlCmd,
			os.Stdin,
			os.Stdout,
			os.Stderr,
			false,
			os.Stderr,
		)
	},
}

func init() {
	RootCmd.AddCommand(curlCmd)
}
