package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/premisan/kubectl-net/pkg/k8s"
)

const shUsage = "kubectl net sh [GLOBAL_FLAGS...] [POD_NAME]"

type shOptions struct {
	flags   commonFlags
	podName string
}

var shCmd = &cobra.Command{
	Use:     "sh [GLOBAL_FLAGS...] [POD_NAME]",
	Aliases: []string{"shell", "bash"},
	Short:   "Attach an interactive troubleshooting shell to a pod's network namespace",
	Long: `Spawn and attach an interactive debug shell (netshoot) sharing the target pod's
network namespace. Allows running any network diagnostics (iperf, nmap, netstat, etc.)
interactively in real time.

All kubectl-net flags (-n, -c, --image, --context, --kubeconfig, -v) must be placed
BEFORE POD_NAME.`,
	Example: `  # Open interactive netshoot shell in pod:
  kubectl net sh -n default my-pod

  # Target pod in another namespace:
  kubectl net sh -n production my-pod`,
	DisableFlagParsing: true,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completePrePodArgs(args, toComplete, nil)
	},
	RunE: func(cmd *cobra.Command, rawArgs []string) error {
		opts, showHelp, err := parseShArgs(rawArgs, defaultCommonFlags())
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

		fmt.Fprintf(os.Stderr, "==> Attaching interactive shell to pod '%s' (namespace: %s)...\n",
			opts.podName, clientCtx.Namespace)

		shellCmd := []string{"/bin/bash"}

		return clientCtx.RunInEphemeral(
			ctx,
			opts.podName,
			resolvedContainer,
			opts.flags.resolveDebugImage(k8s.DefaultDebugImage),
			shellCmd,
			os.Stdin,
			os.Stdout,
			nil,  // Stderr must be nil when TTY is true
			true, // TTY = true for interactive shell
			os.Stderr,
		)
	},
}

func parseShArgs(rawArgs []string, defaults commonFlags) (shOptions, bool, error) {
	flags, podName, postPodArgs, showHelp, err := parsePrePodArgs(rawArgs, defaults, shUsage, nil)
	if err != nil || showHelp {
		return shOptions{}, showHelp, err
	}

	if len(postPodArgs) > 0 {
		for _, arg := range postPodArgs {
			if arg == "-h" || arg == "--help" {
				return shOptions{}, true, nil
			}
			if isGlobalFlagToken(arg) {
				return shOptions{}, false, fmt.Errorf("kubectl-net flag %q must be placed before POD_NAME (e.g. kubectl net sh -n <namespace> %s)", arg, podName)
			}
		}
		return shOptions{}, false, fmt.Errorf("unexpected arguments after POD_NAME: %s (Usage: %s)", strings.Join(postPodArgs, " "), shUsage)
	}

	return shOptions{
		flags:   flags,
		podName: podName,
	}, false, nil
}

func init() {
	RootCmd.AddCommand(shCmd)
}
