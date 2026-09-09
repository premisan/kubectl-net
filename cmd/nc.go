package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/premisan/kubectl-net/pkg/k8s"
)

var (
	ncListen    bool
	ncScan      bool
	ncUDP       bool
	ncTimeout   int
	ncKeepAlive bool
	ncPort      int
)

var ncCmd = &cobra.Command{
	Use:     "nc [POD_NAME] [HOST] [PORT] | [POD_NAME] -l [PORT]",
	Aliases: []string{"netcat"},
	Short:   "Execute netcat (nc) connectivity test or listen on a port inside a target pod",
	Long: `Execute netcat (nc) network tests from inside a Kubernetes pod.
Uses an Ephemeral Debug Container to test TCP/UDP connectivity to another target
or listen on a specific port to accept inbound connections from other pods.

Modes:
  1. Connect / Scan mode:
     Test if a specific target host/IP and port is reachable from the pod.
     Use -z / --scan to perform a quick port open/close check without sending data.

  2. Listen mode (-l, --listen):
     Open a listening port inside the pod to verify if other pods can reach it.
     Use -k / --keep-alive to continue listening after disconnects.`,
	Example: `  # Test TCP connection to an internal service port (zero-I/O scan):
  kubectl net nc my-pod backend-service 8080 -z

  # Test TCP connection to an IP address with 3-second timeout:
  kubectl net nc my-pod 10.244.1.25 3306 -z -w 3

  # Test UDP connection to a DNS server:
  kubectl net nc my-pod 10.96.0.10 53 -u -z

  # Interactive TCP connection to send/receive data:
  kubectl net nc my-pod backend-service 8080

  # Listen on TCP port 8080 inside the pod to receive test connections:
  kubectl net nc my-pod -l 8080

  # Listen on UDP port 5353 inside the pod:
  kubectl net nc my-pod -l 5353 -u

  # Keep listening even after client disconnects:
  kubectl net nc my-pod -l 8080 -k`,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return k8s.CompletePods(toComplete, globalNamespace, globalKubeconfig, globalContext)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}

		podName := args[0]
		var (
			targetHost string
			portStr    string
			proto      = "TCP"
		)

		if ncUDP {
			proto = "UDP"
		}

		// Handle Listen mode vs Connect mode
		if ncListen {
			if len(args) >= 2 {
				portStr = args[1]
			} else if ncPort > 0 {
				portStr = strconv.Itoa(ncPort)
			} else {
				return fmt.Errorf("port is required for listen mode. Usage: kubectl net nc %s -l <PORT>", podName)
			}

			// Validate port number
			p, err := strconv.Atoi(portStr)
			if err != nil || p < 1 || p > 65535 {
				return fmt.Errorf("invalid port number '%s': port must be between 1 and 65535", portStr)
			}
		} else {
			// Connect / Scan mode requires HOST and PORT
			if len(args) < 3 {
				if len(args) == 2 && ncPort > 0 {
					targetHost = args[1]
					portStr = strconv.Itoa(ncPort)
				} else {
					return fmt.Errorf("both HOST and PORT are required for connect mode.\nUsage: kubectl net nc %s <HOST> <PORT> [flags] (or use -l <PORT> for listen mode)", podName)
				}
			} else {
				targetHost = args[1]
				portStr = args[2]
			}

			p, err := strconv.Atoi(portStr)
			if err != nil || p < 1 || p > 65535 {
				return fmt.Errorf("invalid port number '%s': port must be between 1 and 65535", portStr)
			}
		}

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

		// Build nc command
		ncArgs := []string{"nc", "-v"}

		if ncListen {
			ncArgs = append(ncArgs, "-l")
			if ncKeepAlive {
				ncArgs = append(ncArgs, "-k")
			}
			if ncUDP {
				ncArgs = append(ncArgs, "-u")
			}
			ncArgs = append(ncArgs, "-p", portStr)

			fmt.Fprintf(os.Stderr, "==> [%s Listen] Starting listener on %s port %s inside pod '%s' (namespace: %s)...\n",
				proto, proto, portStr, podName, clientCtx.Namespace)
			fmt.Fprintf(os.Stderr, "==> Ready to accept connections. (Press Ctrl+C to terminate)\n")
		} else {
			if ncScan {
				ncArgs = append(ncArgs, "-z")
			}
			if ncUDP {
				ncArgs = append(ncArgs, "-u")
			}
			if ncTimeout > 0 {
				ncArgs = append(ncArgs, "-w", strconv.Itoa(ncTimeout))
			}
			ncArgs = append(ncArgs, targetHost, portStr)

			if ncScan {
				fmt.Fprintf(os.Stderr, "==> [%s Scan] Testing connectivity from pod '%s' (namespace: %s) to '%s:%s'...\n",
					proto, podName, clientCtx.Namespace, targetHost, portStr)
			} else {
				fmt.Fprintf(os.Stderr, "==> [%s Connect] Connecting from pod '%s' (namespace: %s) to '%s:%s'...\n",
					proto, podName, clientCtx.Namespace, targetHost, portStr)
				fmt.Fprintf(os.Stderr, "==> Interactive session opened. Type data or press Ctrl+C to exit.\n")
			}
		}

		if globalVerbose {
			fmt.Fprintf(os.Stderr, "==> Debug command: %s\n", strings.Join(ncArgs, " "))
		}

		return clientCtx.RunInEphemeral(
			ctx,
			podName,
			resolvedContainer,
			globalDebugImage,
			ncArgs,
			os.Stdin,
			os.Stdout,
			os.Stderr,
			false,
			os.Stderr,
		)
	},
}

func init() {
	ncCmd.Flags().BoolVarP(&ncListen, "listen", "l", false, "Listen mode, for inbound connects")
	ncCmd.Flags().BoolVarP(&ncScan, "scan", "z", false, "Zero-I/O mode (used for port scanning / connectivity check)")
	ncCmd.Flags().BoolVarP(&ncUDP, "udp", "u", false, "Use UDP instead of default TCP")
	ncCmd.Flags().IntVarP(&ncTimeout, "timeout", "w", 5, "Connection timeout in seconds")
	ncCmd.Flags().BoolVarP(&ncKeepAlive, "keep-alive", "k", false, "Keep listening after disconnect (listen mode only)")
	ncCmd.Flags().IntVarP(&ncPort, "port", "p", 0, "Local or remote port specification")

	RootCmd.AddCommand(ncCmd)
}
