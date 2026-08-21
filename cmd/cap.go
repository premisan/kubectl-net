package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/premisan/kubectl-net/pkg/capture"
	"github.com/premisan/kubectl-net/pkg/k8s"
)

var capOpts = capture.NewDefaultOptions()

var capCmd = &cobra.Command{
	Use:     "cap [POD_NAME]",
	Aliases: []string{"capture", "sniff"},
	Short:   "Capture network packets from a pod and stream to Wireshark / pcap",
	Long: `Capture network packets from a Kubernetes pod/container in real time.
Streams packets directly into a local Wireshark GUI, saves to a .pcap file,
or outputs to stdout for piping to tshark.`,
	Example: `  # Live packet capture to Wireshark GUI:
  kubectl net cap my-pod -n default

  # Apply BPF filter (HTTP/HTTPS only):
  kubectl net cap my-pod -f "tcp port 80 or tcp port 443"

  # Save to a local .pcap file:
  kubectl net cap my-pod -o ./traffic.pcap

  # Stream to stdout for tshark:
  kubectl net cap my-pod -o - | tshark -r -`,
	Args: cobra.MaximumNArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return k8s.CompletePods(toComplete, globalNamespace, globalKubeconfig, globalContext)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			capOpts.PodName = args[0]
		}

		// Bind persistent global flags
		capOpts.Namespace = globalNamespace
		capOpts.KubeContext = globalContext
		capOpts.Kubeconfig = globalKubeconfig
		if capOpts.ContainerName == "" {
			capOpts.ContainerName = globalContainer
		}
		if capOpts.DebugImage == "" {
			capOpts.DebugImage = globalDebugImage
		}
		capOpts.Verbose = globalVerbose

		if err := capOpts.Validate(); err != nil {
			return err
		}

		runner := capture.NewRunner(capOpts)
		return runner.Run(context.Background())
	},
}

func init() {
	capCmd.Flags().StringVarP(&capOpts.PodName, "pod", "p", "", "Target Pod name")
	capCmd.Flags().StringVarP(&capOpts.ContainerName, "container", "c", "", "Target Container name (defaults to first container)")
	capCmd.Flags().StringVarP(&capOpts.Interface, "interface", "i", "any", "Network interface to capture (e.g. eth0, any)")
	capCmd.Flags().StringVarP(&capOpts.Filter, "filter", "f", "", "tcpdump BPF filter expression (e.g. 'tcp port 80')")
	capCmd.Flags().StringVarP(&capOpts.OutputFile, "output", "o", "", "Output destination: path to .pcap file, '-' for stdout, or omit to launch Wireshark")
	capCmd.Flags().StringVarP((*string)(&capOpts.Mode), "mode", "m", string(capture.ModeAuto), "Capture mode: 'auto', 'ephemeral', or 'direct'")
	capCmd.Flags().StringVar(&capOpts.WiresharkPath, "wireshark-path", "", "Custom path to Wireshark binary")

	_ = capCmd.RegisterFlagCompletionFunc("pod", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return k8s.CompletePods(toComplete, globalNamespace, globalKubeconfig, globalContext)
	})
	_ = capCmd.RegisterFlagCompletionFunc("container", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		targetPod := capOpts.PodName
		if len(args) > 0 {
			targetPod = args[0]
		}
		return k8s.CompleteContainers(targetPod, globalNamespace, globalKubeconfig, globalContext)
	})

	RootCmd.AddCommand(capCmd)
}
