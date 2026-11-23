package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/premisan/kubectl-net/pkg/k8s"
)

const digUsage = "kubectl net dig [GLOBAL_FLAGS...] [POD_NAME] [DIG_OPTIONS...] [DOMAIN_NAME]"

type digOptions struct {
	flags      commonFlags
	podName    string
	domainName string
	recordType string
	dnsServer  string
}

var digCmd = &cobra.Command{
	Use:     "dig [GLOBAL_FLAGS...] [POD_NAME] [DIG_OPTIONS...] [DOMAIN_NAME]",
	Aliases: []string{"dns", "nslookup"},
	Short:   "Execute DNS resolution diagnostics (dig) from inside a target pod",
	Long: `Execute DNS queries from inside a Kubernetes pod to test CoreDNS,
internal service discovery (.svc.cluster.local), or external DNS resolution.

All kubectl-net flags (-n, -c, --image, --context, --kubeconfig, -v) must be placed
BEFORE POD_NAME. Dig options (-t/--type, -s/--server) and domain name must be placed
AFTER POD_NAME.`,
	Example: `  # Query internal Kubernetes service DNS from pod-a:
  kubectl net dig pod-a backend-service.default.svc.cluster.local

  # Query external domain:
  kubectl net dig -n default pod-a google.com

  # Query specific record type (e.g. SRV, TXT, AAAA):
  kubectl net dig pod-a _http._tcp.backend-service.default.svc.cluster.local -t SRV

  # Query a specific DNS server IP directly:
  kubectl net dig -n prod pod-a kubernetes.default.svc.cluster.local -s 10.96.0.10`,
	DisableFlagParsing: true,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completePrePodArgs(args, toComplete, nil)
	},
	RunE: func(cmd *cobra.Command, rawArgs []string) error {
		opts, showHelp, err := parseDigArgs(rawArgs, defaultCommonFlags())
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

		digArgs := []string{"dig"}
		if opts.dnsServer != "" {
			digArgs = append(digArgs, "@"+opts.dnsServer)
		}
		if opts.recordType != "" {
			digArgs = append(digArgs, "-t", opts.recordType)
		}
		digArgs = append(digArgs, opts.domainName)

		fmt.Fprintf(os.Stderr, "==> Executing DNS dig from pod '%s' (namespace: %s) for '%s'...\n",
			opts.podName, clientCtx.Namespace, opts.domainName)

		return clientCtx.RunInEphemeral(
			ctx,
			opts.podName,
			resolvedContainer,
			opts.flags.resolveDebugImage(k8s.DefaultDebugImage),
			digArgs,
			os.Stdin,
			os.Stdout,
			os.Stderr,
			false,
			os.Stderr,
		)
	},
}

func parseDigArgs(rawArgs []string, defaults commonFlags) (digOptions, bool, error) {
	flags, podName, postPodArgs, showHelp, err := parsePrePodArgs(rawArgs, defaults, digUsage, nil)
	if err != nil || showHelp {
		return digOptions{}, showHelp, err
	}

	opts := digOptions{
		flags:      flags,
		podName:    podName,
		recordType: "A",
	}

	fs := pflag.NewFlagSet("dig", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVarP(&opts.recordType, "type", "t", "A", "DNS record type (A, AAAA, CNAME, SRV, TXT, MX, etc.)")
	fs.StringVarP(&opts.dnsServer, "server", "s", "", "Specific DNS server IP to query (e.g. 10.96.0.10, 8.8.8.8)")

	if err := fs.Parse(postPodArgs); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return digOptions{}, true, nil
		}
		return digOptions{}, false, wrapPostPodFlagError("dig", podName, err)
	}

	remaining := fs.Args()
	if len(remaining) != 1 {
		return digOptions{}, false, fmt.Errorf("domain name is required after POD_NAME (Usage: %s)", digUsage)
	}
	opts.domainName = remaining[0]
	return opts, false, nil
}

func init() {
	digCmd.Flags().StringP("type", "t", "A", "DNS record type (A, AAAA, CNAME, SRV, TXT, MX, etc.) (place after POD_NAME)")
	digCmd.Flags().StringP("server", "s", "", "Specific DNS server IP to query (e.g. 10.96.0.10, 8.8.8.8) (place after POD_NAME)")

	RootCmd.AddCommand(digCmd)
}
