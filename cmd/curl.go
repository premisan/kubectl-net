package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/premisan/kubectl-net/pkg/k8s"
)

const curlUsage = "kubectl net curl [GLOBAL_FLAGS...] [POD_NAME] [CURL_OPTIONS...] [URL]"

type curlOptions struct {
	flags    commonFlags
	podName  string
	curlArgs []string
}

var curlCmd = &cobra.Command{
	Use:   "curl [GLOBAL_FLAGS...] [POD_NAME] [CURL_OPTIONS...] [URL]",
	Short: "Execute curl HTTP/HTTPS requests from inside a target pod",
	Long: `Execute curl HTTP/HTTPS network tests from inside a Kubernetes pod.
Uses an Ephemeral Debug Container to test connectivity to other pods, internal services,
or external URLs directly from the target pod's network namespace.

All kubectl-net flags (-n, -c, --image, --context, --kubeconfig, -v) must be placed
BEFORE POD_NAME. Everything AFTER POD_NAME is passed verbatim to curl (-v, -k, -X POST,
-H, -d, -I, -L, -c, -n, --connect-timeout, etc.).`,
	Example: `  # Basic GET request from pod to internal cluster service:
  kubectl net curl my-pod http://backend-service.default.svc:8080/healthz

  # Verbose output with custom headers and method:
  kubectl net curl my-pod -v -X POST -H "Content-Type: application/json" \
    -d '{"key":"value"}' http://backend-service:8080/api

  # Insecure HTTPS request with timeout:
  kubectl net curl my-pod -k -v --connect-timeout 5 https://10.96.0.1:443

  # Target a specific namespace and container (kubectl-net flags before POD_NAME):
  kubectl net curl -n production -c app-container my-pod -I https://google.com`,
	DisableFlagParsing: true,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completePrePodArgs(args, toComplete, nil)
	},
	RunE: func(cmd *cobra.Command, rawArgs []string) error {
		opts, showHelp, err := parseCurlArgs(rawArgs, defaultCommonFlags())
		if err != nil {
			return err
		}
		if showHelp {
			return cmd.Help()
		}

		ctx := context.Background()
		clientCtx, err := k8s.NewClientContext(opts.flags.kubeconfig, opts.flags.kubeContext, opts.flags.namespace)
		if err != nil {
			return fmt.Errorf("failed to configure kubernetes client: %w", err)
		}

		pod, err := clientCtx.GetPod(ctx, opts.podName)
		if err != nil {
			return err
		}

		resolvedContainer, err := k8s.FindTargetContainer(pod, opts.flags.containerName)
		if err != nil {
			return err
		}

		finalCurlCmd := append([]string{"curl"}, opts.curlArgs...)
		fmt.Fprintf(os.Stderr, "==> Executing in pod '%s' (namespace: %s, container: %s): %s\n",
			opts.podName, clientCtx.Namespace, resolvedContainer, strings.Join(finalCurlCmd, " "))

		return clientCtx.RunInEphemeral(
			ctx,
			opts.podName,
			resolvedContainer,
			opts.flags.resolveDebugImage(k8s.DefaultDebugImage),
			finalCurlCmd,
			os.Stdin,
			os.Stdout,
			os.Stderr,
			false,
			os.Stderr,
		)
	},
}

func parseCurlArgs(rawArgs []string, defaults commonFlags) (curlOptions, bool, error) {
	flags, podName, postPodArgs, showHelp, err := parsePrePodArgs(rawArgs, defaults, curlUsage, nil)
	if err != nil || showHelp {
		return curlOptions{}, showHelp, err
	}

	if len(postPodArgs) == 0 {
		return curlOptions{}, false, fmt.Errorf("URL or curl arguments required. Example: kubectl net curl %s http://example.com", podName)
	}

	return curlOptions{
		flags:    flags,
		podName:  podName,
		curlArgs: postPodArgs,
	}, false, nil
}

func init() {
	RootCmd.AddCommand(curlCmd)
}
