package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/premisan/kubectl-net/pkg/k8s"
)

const pingUsage = "kubectl net ping [GLOBAL_FLAGS...] [POD_NAME] [PING_OPTIONS...] [TARGET_HOST_OR_IP]"

type pingOptions struct {
	flags      commonFlags
	podName    string
	targetHost string
	count      int
	timeout    int
}

var pingCmd = &cobra.Command{
	Use:   "ping [GLOBAL_FLAGS...] [POD_NAME] [PING_OPTIONS...] [TARGET_HOST_OR_IP]",
	Short: "Execute ICMP ping test from inside a target pod",
	Long: `Execute ICMP ping connectivity and latency tests from inside a Kubernetes pod.
Uses an Ephemeral Debug Container to test network reachability to other pods, nodes,
gateways, or external hosts (e.g. 8.8.8.8).

All kubectl-net flags (-n, -c, --image, --context, --kubeconfig, -v) must be placed
BEFORE POD_NAME. Ping options (-C/--count, -t/--timeout) and target host must be placed
AFTER POD_NAME.`,
	Example: `  # Ping another pod IP from pod-a (4 packets):
  kubectl net ping pod-a 10.244.1.25

  # Ping external DNS with custom packet count:
  kubectl net ping pod-a 8.8.8.8 -C 10

  # Target pod in a specific namespace (kubectl-net flags before POD_NAME):
  kubectl net ping -n prod pod-a gateway.internal.net -C 10 -t 3`,
	DisableFlagParsing: true,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completePrePodArgs(args, toComplete, nil)
	},
	RunE: func(cmd *cobra.Command, rawArgs []string) error {
		opts, showHelp, err := parsePingArgs(rawArgs, defaultCommonFlags())
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

		pingCmdArgs := []string{"ping", "-c", strconv.Itoa(opts.count)}
		if opts.timeout > 0 {
			pingCmdArgs = append(pingCmdArgs, "-W", strconv.Itoa(opts.timeout))
		}
		pingCmdArgs = append(pingCmdArgs, opts.targetHost)

		fmt.Fprintf(os.Stderr, "==> Executing ping from pod '%s' (namespace: %s) to '%s'...\n",
			opts.podName, clientCtx.Namespace, opts.targetHost)

		return clientCtx.RunInEphemeral(
			ctx,
			opts.podName,
			resolvedContainer,
			opts.flags.resolveDebugImage(k8s.DefaultDebugImage),
			pingCmdArgs,
			os.Stdin,
			os.Stdout,
			os.Stderr,
			false,
			os.Stderr,
		)
	},
}

func parsePingArgs(rawArgs []string, defaults commonFlags) (pingOptions, bool, error) {
	flags, podName, postPodArgs, showHelp, err := parsePrePodArgs(rawArgs, defaults, pingUsage, nil)
	if err != nil || showHelp {
		return pingOptions{}, showHelp, err
	}

	opts := pingOptions{
		flags:   flags,
		podName: podName,
		count:   4,
		timeout: 5,
	}

	fs := pflag.NewFlagSet("ping", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.IntVarP(&opts.count, "count", "C", 4, "Number of ICMP ECHO packets to send")
	fs.IntVarP(&opts.timeout, "timeout", "t", 5, "Time to wait for a response in seconds")

	if err := fs.Parse(postPodArgs); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return pingOptions{}, true, nil
		}
		return pingOptions{}, false, wrapPostPodFlagError("ping", podName, err)
	}

	remaining := fs.Args()
	if len(remaining) != 1 {
		return pingOptions{}, false, fmt.Errorf("target host or IP is required after POD_NAME (Usage: %s)", pingUsage)
	}
	opts.targetHost = remaining[0]
	return opts, false, nil
}

func init() {
	pingCmd.Flags().IntP("count", "C", 4, "Number of ICMP ECHO packets to send (place after POD_NAME)")
	pingCmd.Flags().IntP("timeout", "t", 5, "Time to wait for a response in seconds (place after POD_NAME)")

	RootCmd.AddCommand(pingCmd)
}
