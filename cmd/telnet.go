package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/premisan/kubectl-net/pkg/k8s"
)

const telnetUsage = "kubectl net telnet [GLOBAL_FLAGS...] [POD_NAME] [TELNET_OPTIONS...] [HOST] [PORT]"

type telnetOptions struct {
	flags      commonFlags
	podName    string
	telnetArgs []string
}

var telnetCmd = &cobra.Command{
	Use:   "telnet [GLOBAL_FLAGS...] [POD_NAME] [TELNET_OPTIONS...] [HOST] [PORT]",
	Short: "Execute telnet TCP connectivity test from inside a target pod",
	Long: `Execute telnet TCP connectivity and banner diagnostics from inside a Kubernetes pod
using an Ephemeral Debug Container (default image: nicolaka/netshoot:latest, which bundles
telnet via busybox-extras).

All kubectl-net flags (-n, -c, --image, --context, --kubeconfig, -v) must be placed
BEFORE POD_NAME. Everything AFTER POD_NAME is passed verbatim to telnet.

Interactive sessions automatically allocate a TTY; piped or redirected I/O automatically
switches to non-TTY mode.`,
	Example: `  # Test TCP connectivity to an internal service port:
  kubectl net telnet my-pod backend-service 8080

  # Test connectivity from a pod in a specific namespace:
  kubectl net telnet -n production my-pod 10.244.1.25 3306

  # Test SMTP/HTTP banner interactively (press Ctrl+] then 'quit' to exit):
  kubectl net telnet -n default my-pod mail.internal 25`,
	DisableFlagParsing: true,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completePrePodArgs(args, toComplete, nil)
	},
	RunE: func(cmd *cobra.Command, rawArgs []string) error {
		opts, showHelp, err := parseTelnetArgs(rawArgs, defaultCommonFlags())
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

		finalTelnetCmd := append([]string{"telnet"}, opts.telnetArgs...)
		fmt.Fprintf(os.Stderr, "==> Executing in pod '%s' (namespace: %s, container: %s): %s\n",
			opts.podName, clientCtx.Namespace, resolvedContainer, strings.Join(finalTelnetCmd, " "))

		useTTY := term.IsTerminal(int(os.Stdin.Fd())) &&
			term.IsTerminal(int(os.Stdout.Fd()))

		var stderrWriter io.Writer = os.Stderr
		if useTTY {
			stderrWriter = nil
		}

		return clientCtx.RunInEphemeral(
			ctx,
			opts.podName,
			resolvedContainer,
			opts.flags.resolveDebugImage(k8s.DefaultDebugImage),
			finalTelnetCmd,
			os.Stdin,
			os.Stdout,
			stderrWriter,
			useTTY,
			os.Stderr,
		)
	},
}

func parseTelnetArgs(rawArgs []string, defaults commonFlags) (telnetOptions, bool, error) {
	flags, podName, postPodArgs, showHelp, err := parsePrePodArgs(rawArgs, defaults, telnetUsage, nil)
	if err != nil || showHelp {
		return telnetOptions{}, showHelp, err
	}

	if len(postPodArgs) == 0 {
		return telnetOptions{}, false, fmt.Errorf("target host (and optional port) is required after POD_NAME. Example: kubectl net telnet %s backend-service 8080", podName)
	}

	return telnetOptions{
		flags:      flags,
		podName:    podName,
		telnetArgs: postPodArgs,
	}, false, nil
}

func init() {
	RootCmd.AddCommand(telnetCmd)
}
