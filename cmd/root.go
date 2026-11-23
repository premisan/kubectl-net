package cmd

import (
	"fmt"
	"os"
	"path/filepath"

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
	Short: "kubectl-net - Comprehensive Kubernetes network debugging CLI toolkit",
	Long: `kubectl-net is a versatile Kubernetes network debugging toolkit.
It uses Ephemeral Debug Containers to bypass container runtime limitations (containerd / GKE)
and works seamlessly on distroless/scratch containers without modifying target workloads.

Unified Argument Convention:
  kubectl net <subcommand> [KUBECTL_NET_FLAGS...] <POD_NAME> [COMMAND_ARGS...]

  Place all kubectl-net flags (-n/--namespace, -c/--container, --image, --context,
  --kubeconfig, -v/--verbose, and -i/--interval for while) BEFORE POD_NAME.
  Everything AFTER POD_NAME belongs strictly to the target command.

Subcommands:
  cap   - Live packet capture into Wireshark, .pcap file, or stdout
  curl  - Execute curl HTTP/HTTPS requests from inside a target pod
  ping  - Execute ICMP ping tests from inside a target pod
  dig   - Perform DNS resolution diagnostics (dig/nslookup) from inside a pod
  nc     - Execute netcat (nc) port connectivity tests or listen on a port
  telnet - Execute telnet TCP connectivity and banner tests from inside a pod
  sh     - Attach an interactive debug shell (netshoot) to a pod's network namespace
  while  - Repeatedly execute a command at a fixed interval inside a target pod
  mysql  - Connect to a MySQL server from inside a target pod (default image: mysql:latest)
  psql   - Connect to a PostgreSQL server from inside a target pod (default image: postgres:latest)
  redis  - Connect to a Redis server from inside a target pod (default image: redis:latest)`,
	Example: `  # Live packet capture to Wireshark:
  kubectl net cap -n default my-pod -f "port 80"

  # Curl test from pod to internal service or external URL:
  kubectl net curl -n production my-pod -v http://backend-svc:8080/healthz

  # Ping test from pod to another pod IP or gateway:
  kubectl net ping -n default my-pod 10.244.1.25

  # DNS diagnostic:
  kubectl net dig -n default my-pod backend-svc.default.svc.cluster.local

  # Port connectivity check (zero-I/O scan):
  kubectl net nc my-pod backend-svc 8080 -z

  # Listen on port inside pod to accept incoming test connections:
  kubectl net nc my-pod -l 8080

  # Telnet TCP connectivity test from pod:
  kubectl net telnet -n default my-pod backend-svc 8080

  # Interactive network troubleshooting shell:
  kubectl net sh -n default my-pod

  # Repeat a command inside pod every 5 seconds:
  kubectl net while -n default -i 5 my-pod curl -v https://ifconfig.me/

  # Connect to MySQL, PostgreSQL, or Redis from inside a pod:
  kubectl net mysql -n production my-pod -h mysql-svc -u admin -p mydb
  kubectl net psql  -n production my-pod -h postgres-svc -U postgres -d mydb
  kubectl net redis -n production my-pod -h redis-svc -p 6379 PING`,
	SilenceUsage: true,
}

func init() {
	RootCmd.PersistentFlags().StringVarP(&globalNamespace, "namespace", "n", "", "Kubernetes namespace (defaults to current context namespace)")
	RootCmd.PersistentFlags().StringVar(&globalContext, "context", "", "The name of the kubeconfig context to use")
	RootCmd.PersistentFlags().StringVar(&globalKubeconfig, "kubeconfig", "", "Path to the kubeconfig file")
	RootCmd.PersistentFlags().StringVarP(&globalContainer, "container", "c", "", "Target Container name (defaults to first container)")
	RootCmd.PersistentFlags().StringVar(&globalDebugImage, "image", "", "Image to use for Ephemeral debug container (defaults: nicolaka/netshoot:latest; mysql:latest, postgres:latest, redis:latest for DB subcommands)")
	RootCmd.PersistentFlags().BoolVarP(&globalVerbose, "verbose", "v", false, "Enable verbose logging")

	_ = RootCmd.RegisterFlagCompletionFunc("namespace", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return k8s.CompleteNamespaces(toComplete, globalKubeconfig, globalContext)
	})

	RootCmd.Version = version
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if len(os.Args) > 0 && filepath.Base(os.Args[0]) == "kubectl_complete-net" {
		RootCmd.SetArgs(append([]string{"__complete"}, os.Args[1:]...))
	}
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
