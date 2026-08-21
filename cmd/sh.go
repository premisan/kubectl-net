package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/premisan/kubectl-net/pkg/k8s"
)

var shCmd = &cobra.Command{
	Use:     "sh [POD_NAME]",
	Aliases: []string{"shell", "bash"},
	Short:   "Attach an interactive troubleshooting shell to a pod's network namespace",
	Long: `Spawn and attach an interactive debug shell (netshoot) sharing the target pod's
network namespace. Allows running any network diagnostics (iperf, nmap, netstat, etc.)
interactively in real time.`,
	Example: `  # Open interactive netshoot shell in pod:
  kubectl net sh my-pod -n default

  # Target pod in another namespace:
  kubectl net sh my-pod -n production`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		podName := args[0]

		ctx := context.Background()
		clientCtx, err := k8s.NewClientContext(globalKubeconfig, globalContext, globalNamespace)
		if err != nil {
			return fmt.Errorf("failed to configure kubernetes client: %w", err)
		}

		pod, err := clientCtx.GetPod(ctx, podName)
		if err != nil {
			return err
		}

		resolvedContainer, err := k8s.FindTargetContainer(pod, globalContainer)
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "==> Attaching interactive shell to pod '%s' (namespace: %s)...\n",
			podName, clientCtx.Namespace)

		// Try bash first, if not found fallback is handled in container
		shellCmd := []string{"/bin/bash"}

		return clientCtx.RunInEphemeral(
			ctx,
			podName,
			resolvedContainer,
			globalDebugImage,
			shellCmd,
			os.Stdin,
			os.Stdout,
			os.Stderr,
			true, // TTY = true for interactive shell
			os.Stderr,
		)
	},
}

func init() {
	RootCmd.AddCommand(shCmd)
}
