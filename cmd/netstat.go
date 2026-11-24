package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/premisan/kubectl-net/pkg/k8s"
)

const netstatUsage = "kubectl net netstat [GLOBAL_FLAGS...] [POD_NAME] [NETSTAT_OPTIONS...]"

type netstatOptions struct {
	flags       commonFlags
	podName     string
	netstatArgs []string
}

var netstatCmd = &cobra.Command{
	Use:   "netstat [GLOBAL_FLAGS...] [POD_NAME] [NETSTAT_OPTIONS...]",
	Short: "Execute netstat to inspect sockets, listening ports, and routing tables inside a target pod",
	Long: `Execute netstat from inside a Kubernetes pod using an Ephemeral Debug Container
(default image: nicolaka/netshoot:latest) sharing the target pod's network namespace
and target container's process namespace (enabling -p PID/program inspection).

All kubectl-net flags (-n, -c, --image, --context, --kubeconfig, -v) must be placed
BEFORE POD_NAME. Everything AFTER POD_NAME is passed verbatim to netstat (-tulnp,
-antp, -rn, -s, -i, -c, -n, etc.). Running with no options after POD_NAME invokes
plain 'netstat'.`,
	Example: `  # List listening TCP/UDP ports with numeric addresses and PID/program names:
  kubectl net netstat my-pod -tulnp

  # Show all TCP connections from a pod in a specific namespace:
  kubectl net netstat -n production my-pod -antp

  # Show the pod's kernel routing table:
  kubectl net netstat my-pod -rn

  # Show network protocol statistics or interface counters:
  kubectl net netstat my-pod -s
  kubectl net netstat my-pod -i

  # Continuously monitor TCP connections (press Ctrl+C to stop):
  kubectl net netstat -n default my-pod -c -ant

  # Run default netstat output:
  kubectl net netstat my-pod`,
	DisableFlagParsing: true,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completePrePodArgs(args, toComplete, nil)
	},
	RunE: func(cmd *cobra.Command, rawArgs []string) error {
		opts, showHelp, err := parseNetstatArgs(rawArgs, defaultCommonFlags())
		if err != nil {
			return err
		}
		if showHelp {
			return cmd.Help()
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

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

		finalNetstatCmd := append([]string{"netstat"}, opts.netstatArgs...)
		fmt.Fprintf(os.Stderr, "==> Executing in pod '%s' (namespace: %s, container: %s): %s\n",
			opts.podName, clientCtx.Namespace, resolvedContainer, strings.Join(finalNetstatCmd, " "))

		return clientCtx.RunInEphemeral(
			ctx,
			opts.podName,
			resolvedContainer,
			opts.flags.resolveDebugImage(k8s.DefaultDebugImage),
			finalNetstatCmd,
			nil,
			os.Stdout,
			os.Stderr,
			false,
			os.Stderr,
		)
	},
}

func parseNetstatArgs(rawArgs []string, defaults commonFlags) (netstatOptions, bool, error) {
	flags, podName, postPodArgs, showHelp, err := parsePrePodArgs(rawArgs, defaults, netstatUsage, nil)
	if err != nil || showHelp {
		return netstatOptions{}, showHelp, err
	}

	return netstatOptions{
		flags:       flags,
		podName:     podName,
		netstatArgs: postPodArgs,
	}, false, nil
}

func init() {
	RootCmd.AddCommand(netstatCmd)
}
