package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/premisan/kubectl-net/pkg/k8s"
)

var (
	pingCount   int
	pingTimeout int
)

var pingCmd = &cobra.Command{
	Use:   "ping [POD_NAME] [TARGET_HOST_OR_IP]",
	Short: "Execute ICMP ping test from inside a target pod",
	Long: `Execute ICMP ping connectivity and latency tests from inside a Kubernetes pod.
Uses an Ephemeral Debug Container to test network reachability to other pods, nodes,
gateways, or external hosts (e.g. 8.8.8.8).`,
	Example: `  # Ping another pod IP from pod-a (4 packets):
  kubectl net ping pod-a 10.244.1.25

  # Ping external DNS with custom packet count:
  kubectl net ping pod-a 8.8.8.8 -c 10

  # Target pod in a specific namespace:
  kubectl net ping pod-a gateway.internal.net -n prod`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		podName := args[0]
		targetHost := args[1]

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

		pingCmdArgs := []string{"ping", "-c", strconv.Itoa(pingCount)}
		if pingTimeout > 0 {
			pingCmdArgs = append(pingCmdArgs, "-W", strconv.Itoa(pingTimeout))
		}
		pingCmdArgs = append(pingCmdArgs, targetHost)

		fmt.Fprintf(os.Stderr, "==> Executing ping from pod '%s' (namespace: %s) to '%s'...\n",
			podName, clientCtx.Namespace, targetHost)

		return clientCtx.RunInEphemeral(
			ctx,
			podName,
			resolvedContainer,
			globalDebugImage,
			pingCmdArgs,
			os.Stdin,
			os.Stdout,
			os.Stderr,
			false,
			os.Stderr,
		)
	},
}

func init() {
	pingCmd.Flags().IntVarP(&pingCount, "count", "C", 4, "Number of ICMP ECHO packets to send")
	pingCmd.Flags().IntVarP(&pingTimeout, "timeout", "t", 5, "Time to wait for a response in seconds")

	RootCmd.AddCommand(pingCmd)
}
