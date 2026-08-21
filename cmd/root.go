package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"kcap/pkg/capture"
)

var (
	opts    = capture.NewDefaultOptions()
	version = "0.1.0"
)

// RootCmd represents the base command
var RootCmd = &cobra.Command{
	Use:   "kcap [POD_NAME]",
	Short: "kcap - Modern Kubernetes packet capture tool for Wireshark (containerd & GKE ready)",
	Long: `kcap is a lightweight CLI tool to capture network packets from Kubernetes Pod containers
and stream them directly into Wireshark, a local pcap file, or stdout.

Built with native Kubernetes Ephemeral Container support to work reliably across
all containerd versions, GKE (Container-Optimized OS), and on-premise clusters
without requiring docker/containerd runtime socket mounts.`,
	Example: `  # Capture traffic on a pod and stream directly into Wireshark GUI:
  kcap my-pod -n default

  # Target a specific container and apply a BPF packet filter:
  kcap my-pod -c app-container -n prod -f "tcp port 80 or tcp port 443"

  # Save capture stream to a local .pcap file (without opening Wireshark):
  kcap my-pod -o ./capture.pcap -f "host 10.0.0.1"

  # Stream raw pcap to stdout for piping to tshark:
  kcap my-pod -o - | tshark -r -

  # Explicitly specify ephemeral debug container image:
  kcap my-pod --mode ephemeral --image nicolaka/netshoot:latest`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			opts.PodName = args[0]
		}

		if err := opts.Validate(); err != nil {
			return err
		}

		runner := capture.NewRunner(opts)
		return runner.Run(context.Background())
	},
}

func init() {
	RootCmd.Flags().StringVarP(&opts.PodName, "pod", "p", "", "Target Pod name")
	RootCmd.Flags().StringVarP(&opts.ContainerName, "container", "c", "", "Target Container name (defaults to first container)")
	RootCmd.Flags().StringVarP(&opts.Namespace, "namespace", "n", "", "Kubernetes namespace (defaults to current context namespace)")
	RootCmd.Flags().StringVarP(&opts.Interface, "interface", "i", "any", "Network interface to capture (e.g. eth0, any)")
	RootCmd.Flags().StringVarP(&opts.Filter, "filter", "f", "", "tcpdump BPF filter expression (e.g. 'tcp port 80')")
	RootCmd.Flags().StringVarP(&opts.OutputFile, "output", "o", "", "Output destination: path to .pcap file, '-' for stdout, or omit to launch Wireshark")
	RootCmd.Flags().StringVarP((*string)(&opts.Mode), "mode", "m", string(capture.ModeAuto), "Capture mode: 'auto', 'ephemeral', or 'direct'")
	RootCmd.Flags().StringVar(&opts.DebugImage, "image", "nicolaka/netshoot:latest", "Image to use for Ephemeral debug container")
	RootCmd.Flags().StringVar(&opts.WiresharkPath, "wireshark-path", "", "Custom path to Wireshark binary")
	RootCmd.Flags().StringVar(&opts.Kubeconfig, "kubeconfig", "", "Path to the kubeconfig file")
	RootCmd.Flags().StringVar(&opts.KubeContext, "context", "", "The name of the kubeconfig context to use")
	RootCmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", false, "Enable verbose logging")

	RootCmd.Version = version
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
