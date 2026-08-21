package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/premisan/kubectl-net/pkg/k8s"
)

var (
	digType   string
	digServer string
)

var digCmd = &cobra.Command{
	Use:     "dig [POD_NAME] [DOMAIN_NAME]",
	Aliases: []string{"dns", "nslookup"},
	Short:   "Execute DNS resolution diagnostics (dig) from inside a target pod",
	Long: `Execute DNS queries from inside a Kubernetes pod to test CoreDNS,
internal service discovery (.svc.cluster.local), or external DNS resolution.`,
	Example: `  # Query internal Kubernetes service DNS from pod-a:
  kubectl net dig pod-a backend-service.default.svc.cluster.local

  # Query external domain:
  kubectl net dig pod-a google.com

  # Query specific record type (e.g. SRV, TXT, AAAA):
  kubectl net dig pod-a _http._tcp.backend-service.default.svc.cluster.local -t SRV

  # Query a specific DNS server IP directly:
  kubectl net dig pod-a kubernetes.default.svc.cluster.local -s 10.96.0.10`,
	Args: cobra.ExactArgs(2),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return k8s.CompletePods(toComplete, globalNamespace, globalKubeconfig, globalContext)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		podName := args[0]
		domainName := args[1]

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

		digArgs := []string{"dig"}
		if digServer != "" {
			digArgs = append(digArgs, "@"+digServer)
		}
		if digType != "" {
			digArgs = append(digArgs, "-t", digType)
		}
		digArgs = append(digArgs, domainName)

		fmt.Fprintf(os.Stderr, "==> Executing DNS dig from pod '%s' (namespace: %s) for '%s'...\n",
			podName, clientCtx.Namespace, domainName)

		return clientCtx.RunInEphemeral(
			ctx,
			podName,
			resolvedContainer,
			globalDebugImage,
			digArgs,
			os.Stdin,
			os.Stdout,
			os.Stderr,
			false,
			os.Stderr,
		)
	},
}

func init() {
	digCmd.Flags().StringVarP(&digType, "type", "t", "A", "DNS record type (A, AAAA, CNAME, SRV, TXT, MX, etc.)")
	digCmd.Flags().StringVarP(&digServer, "server", "s", "", "Specific DNS server IP to query (e.g. 10.96.0.10, 8.8.8.8)")

	RootCmd.AddCommand(digCmd)
}
