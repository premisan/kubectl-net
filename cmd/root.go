package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	utilexec "k8s.io/client-go/util/exec"

	"github.com/premisan/kubectl-net/pkg/k8s"
)

var (
	// Global flags
	globalNamespace  string
	globalContext    string
	globalKubeconfig string
	globalContainer  string
	globalDebugImage string
	globalVerbose    bool

	version = "0.2.0"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "kubectl-net [command]",
	Short: "kubectl-net (knet) - Comprehensive Kubernetes network debugging CLI toolkit",
	Long: `kubectl-net (knet) is a versatile Kubernetes network debugging toolkit.
It uses Ephemeral Debug Containers to bypass container runtime limitations (containerd / GKE)
and works seamlessly on distroless/scratch containers without modifying target workloads.

Subcommands:
  cap   - Live packet capture into Wireshark, .pcap file, or stdout
  curl  - Execute curl HTTP/HTTPS requests from inside a target pod
  ping  - Execute ICMP ping tests from inside a target pod
  dig   - Perform DNS resolution diagnostics (dig/nslookup) from inside a pod
  nc    - Execute netcat (nc) port connectivity tests or listen on a port
  sh    - Attach an interactive debug shell (netshoot) to a pod's network namespace`,
	Example: `  # Live packet capture to Wireshark:
  kubectl net cap my-pod -n default -f "port 80"

  # Curl test from pod to internal service or external URL:
  kubectl net curl my-pod -v http://backend-svc:8080/healthz

  # Ping test from pod to another pod IP or gateway:
  kubectl net ping my-pod 10.244.1.25

  # DNS diagnostic:
  kubectl net dig my-pod backend-svc.default.svc.cluster.local

  # Port connectivity check (zero-I/O scan):
  kubectl net nc my-pod backend-svc 8080 -z

  # Listen on port inside pod to accept incoming test connections:
  kubectl net nc my-pod -l 8080

  # Interactive network troubleshooting shell:
  kubectl net sh my-pod -n default`,
	SilenceUsage: true,
}

func init() {
	RootCmd.PersistentFlags().StringVarP(&globalNamespace, "namespace", "n", "", "Kubernetes namespace (defaults to current context namespace)")
	RootCmd.PersistentFlags().StringVar(&globalContext, "context", "", "The name of the kubeconfig context to use")
	RootCmd.PersistentFlags().StringVar(&globalKubeconfig, "kubeconfig", "", "Path to the kubeconfig file")
	RootCmd.PersistentFlags().StringVarP(&globalContainer, "container", "c", "", "Target Container name (defaults to first container)")
	RootCmd.PersistentFlags().StringVar(&globalDebugImage, "image", "nicolaka/netshoot:latest", "Image to use for Ephemeral debug container")
	RootCmd.PersistentFlags().BoolVarP(&globalVerbose, "verbose", "v", false, "Enable verbose logging")

	_ = RootCmd.RegisterFlagCompletionFunc("namespace", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return k8s.CompleteNamespaces(toComplete, globalKubeconfig, globalContext)
	})

	RootCmd.Version = version
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	RootCmd.SilenceErrors = true
	if err := RootCmd.Execute(); err != nil {
		if exitErr, ok := err.(utilexec.CodeExitError); ok {
			os.Exit(exitErr.Code)
		}
		if exitErr, ok := err.(utilexec.ExitError); ok && exitErr.Exited() {
			os.Exit(exitErr.ExitStatus())
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
