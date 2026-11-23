package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/premisan/kubectl-net/pkg/k8s"
)

const ncUsage = "kubectl net nc [GLOBAL_FLAGS...] [POD_NAME] [NC_OPTIONS...] [HOST] [PORT] | [GLOBAL_FLAGS...] [POD_NAME] -l [PORT]"

type ncOptions struct {
	flags      commonFlags
	podName    string
	targetHost string
	portStr    string
	listen     bool
	scan       bool
	udp        bool
	timeout    int
	keepAlive  bool
	port       int
}

var ncCmd = &cobra.Command{
	Use:     "nc [GLOBAL_FLAGS...] [POD_NAME] [NC_OPTIONS...] [HOST] [PORT] | [GLOBAL_FLAGS...] [POD_NAME] -l [PORT]",
	Aliases: []string{"netcat"},
	Short:   "Execute netcat (nc) connectivity test or listen on a port inside a target pod",
	Long: `Execute netcat (nc) network tests from inside a Kubernetes pod.
Uses an Ephemeral Debug Container to test TCP/UDP connectivity to another target
or listen on a specific port to accept inbound connections from other pods.

All kubectl-net flags (-n, -c, --image, --context, --kubeconfig, -v) must be placed
BEFORE POD_NAME. All netcat options (-l, -z, -u, -w, -k, -p) and target host/port
must be placed AFTER POD_NAME.

Modes:
  1. Connect / Scan mode:
     Test if a specific target host/IP and port is reachable from the pod.
     Use -z / --scan to perform a quick port open/close check without sending data.

  2. Listen mode (-l, --listen):
     Open a listening port inside the pod to verify if other pods can reach it.
     Use -k / --keep-alive to continue listening after disconnects.`,
	Example: `  # Test TCP connection to an internal service port (zero-I/O scan):
  kubectl net nc my-pod backend-service 8080 -z

  # Test TCP connection from a pod in a specific namespace:
  kubectl net nc -n prod my-pod 10.244.1.25 3306 -z -w 3

  # Test UDP connection to a DNS server:
  kubectl net nc my-pod 10.96.0.10 53 -u -z

  # Interactive TCP connection to send/receive data:
  kubectl net nc my-pod backend-service 8080

  # Listen on TCP port 8080 inside the pod to receive test connections:
  kubectl net nc -n default my-pod -l 8080

  # Listen on UDP port 5353 inside the pod:
  kubectl net nc my-pod -l 5353 -u

  # Keep listening even after client disconnects:
  kubectl net nc my-pod -l 8080 -k`,
	DisableFlagParsing: true,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completePrePodArgs(args, toComplete, nil)
	},
	RunE: func(cmd *cobra.Command, rawArgs []string) error {
		opts, showHelp, err := parseNcArgs(rawArgs, defaultCommonFlags())
		if err != nil {
			return err
		}
		if showHelp {
			return cmd.Help()
		}

		proto := "TCP"
		if opts.udp {
			proto = "UDP"
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

		// Build nc command
		ncArgs := []string{"nc", "-v"}

		if opts.listen {
			ncArgs = append(ncArgs, "-l")
			if opts.keepAlive {
				ncArgs = append(ncArgs, "-k")
			}
			if opts.udp {
				ncArgs = append(ncArgs, "-u")
			}
			ncArgs = append(ncArgs, "-p", opts.portStr)

			fmt.Fprintf(os.Stderr, "==> [%s Listen] Starting listener on %s port %s inside pod '%s' (namespace: %s)...\n",
				proto, proto, opts.portStr, opts.podName, clientCtx.Namespace)
			fmt.Fprintf(os.Stderr, "==> Ready to accept connections. (Press Ctrl+C to terminate)\n")
		} else {
			if opts.scan {
				ncArgs = append(ncArgs, "-z")
			}
			if opts.udp {
				ncArgs = append(ncArgs, "-u")
			}
			if opts.timeout > 0 {
				ncArgs = append(ncArgs, "-w", strconv.Itoa(opts.timeout))
			}
			ncArgs = append(ncArgs, opts.targetHost, opts.portStr)

			if opts.scan {
				fmt.Fprintf(os.Stderr, "==> [%s Scan] Testing connectivity from pod '%s' (namespace: %s) to '%s:%s'...\n",
					proto, opts.podName, clientCtx.Namespace, opts.targetHost, opts.portStr)
			} else {
				fmt.Fprintf(os.Stderr, "==> [%s Connect] Connecting from pod '%s' (namespace: %s) to '%s:%s'...\n",
					proto, opts.podName, clientCtx.Namespace, opts.targetHost, opts.portStr)
				fmt.Fprintf(os.Stderr, "==> Interactive session opened. Type data or press Ctrl+C to exit.\n")
			}
		}

		if opts.flags.verbose {
			fmt.Fprintf(os.Stderr, "==> Debug command: %s\n", strings.Join(ncArgs, " "))
		}

		return clientCtx.RunInEphemeral(
			ctx,
			opts.podName,
			resolvedContainer,
			opts.flags.resolveDebugImage(k8s.DefaultDebugImage),
			ncArgs,
			os.Stdin,
			os.Stdout,
			os.Stderr,
			false,
			os.Stderr,
		)
	},
}

func parseNcArgs(rawArgs []string, defaults commonFlags) (ncOptions, bool, error) {
	flags, podName, postPodArgs, showHelp, err := parsePrePodArgs(rawArgs, defaults, ncUsage, nil)
	if err != nil || showHelp {
		return ncOptions{}, showHelp, err
	}

	opts := ncOptions{
		flags:   flags,
		podName: podName,
		timeout: 5,
	}

	fs := pflag.NewFlagSet("nc", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVarP(&opts.listen, "listen", "l", false, "Listen mode, for inbound connects")
	fs.BoolVarP(&opts.scan, "scan", "z", false, "Zero-I/O mode (used for port scanning / connectivity check)")
	fs.BoolVarP(&opts.udp, "udp", "u", false, "Use UDP instead of default TCP")
	fs.IntVarP(&opts.timeout, "timeout", "w", 5, "Connection timeout in seconds")
	fs.BoolVarP(&opts.keepAlive, "keep-alive", "k", false, "Keep listening after disconnect (listen mode only)")
	fs.IntVarP(&opts.port, "port", "p", 0, "Local or remote port specification")

	if err := fs.Parse(postPodArgs); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return ncOptions{}, true, nil
		}
		return ncOptions{}, false, wrapPostPodFlagError("nc", podName, err)
	}

	remaining := fs.Args()

	if opts.listen {
		if len(remaining) >= 1 {
			opts.portStr = remaining[0]
		} else if opts.port > 0 {
			opts.portStr = strconv.Itoa(opts.port)
		} else {
			return ncOptions{}, false, fmt.Errorf("port is required for listen mode. Usage: kubectl net nc %s -l <PORT>", podName)
		}

		p, err := strconv.Atoi(opts.portStr)
		if err != nil || p < 1 || p > 65535 {
			return ncOptions{}, false, fmt.Errorf("invalid port number '%s': port must be between 1 and 65535", opts.portStr)
		}
		return opts, false, nil
	}

	if len(remaining) < 2 {
		if len(remaining) == 1 && opts.port > 0 {
			opts.targetHost = remaining[0]
			opts.portStr = strconv.Itoa(opts.port)
		} else {
			return ncOptions{}, false, fmt.Errorf("both HOST and PORT are required for connect mode.\nUsage: kubectl net nc %s <HOST> <PORT> [flags] (or use -l <PORT> for listen mode)", podName)
		}
	} else {
		opts.targetHost = remaining[0]
		opts.portStr = remaining[1]
	}

	p, err := strconv.Atoi(opts.portStr)
	if err != nil || p < 1 || p > 65535 {
		return ncOptions{}, false, fmt.Errorf("invalid port number '%s': port must be between 1 and 65535", opts.portStr)
	}

	return opts, false, nil
}

func init() {
	ncCmd.Flags().BoolP("listen", "l", false, "Listen mode, for inbound connects (place after POD_NAME)")
	ncCmd.Flags().BoolP("scan", "z", false, "Zero-I/O mode (used for port scanning / connectivity check) (place after POD_NAME)")
	ncCmd.Flags().BoolP("udp", "u", false, "Use UDP instead of default TCP (place after POD_NAME)")
	ncCmd.Flags().IntP("timeout", "w", 5, "Connection timeout in seconds (place after POD_NAME)")
	ncCmd.Flags().BoolP("keep-alive", "k", false, "Keep listening after disconnect (listen mode only) (place after POD_NAME)")
	ncCmd.Flags().IntP("port", "p", 0, "Local or remote port specification (place after POD_NAME)")

	RootCmd.AddCommand(ncCmd)
}
