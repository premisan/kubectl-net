package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/premisan/kubectl-net/pkg/capture"
	"github.com/premisan/kubectl-net/pkg/k8s"
)

const capUsage = "kubectl net cap [GLOBAL_FLAGS...] [POD_NAME] [CAP_OPTIONS...]"

var capCmd = &cobra.Command{
	Use:     "cap [GLOBAL_FLAGS...] [POD_NAME] [CAP_OPTIONS...]",
	Aliases: []string{"capture", "sniff"},
	Short:   "Capture network packets from a pod and stream to Wireshark / pcap",
	Long: `Capture network packets from a Kubernetes pod/container in real time.
Streams packets directly into a local Wireshark GUI, saves to a .pcap file,
or outputs to stdout for piping to tshark.

All kubectl-net flags (-n, -c, --image, --context, --kubeconfig, -v) must be placed
BEFORE POD_NAME. Capture options (-i, -f, -o, -m, --wireshark-path) must be placed
AFTER POD_NAME.`,
	Example: `  # Live packet capture to Wireshark GUI:
  kubectl net cap -n default my-pod

  # Apply BPF filter (HTTP/HTTPS only):
  kubectl net cap -n default my-pod -f "tcp port 80 or tcp port 443"

  # Save to a local .pcap file:
  kubectl net cap my-pod -o ./traffic.pcap

  # Stream to stdout for tshark:
  kubectl net cap my-pod -o - | tshark -r -`,
	DisableFlagParsing: true,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completePrePodArgs(args, toComplete, nil)
	},
	RunE: func(cmd *cobra.Command, rawArgs []string) error {
		opts, showHelp, err := parseCapArgs(rawArgs, defaultCommonFlags())
		if err != nil {
			return err
		}
		if showHelp {
			return cmd.Help()
		}

		if err := opts.Validate(); err != nil {
			return err
		}

		runner := capture.NewRunner(opts)
		return runner.Run(context.Background())
	},
}

func newCapFlagSet(opts *capture.Options) *pflag.FlagSet {
	fs := pflag.NewFlagSet("cap", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVarP(&opts.Interface, "interface", "i", "any", "Network interface to capture (e.g. eth0, any)")
	fs.StringVarP(&opts.Filter, "filter", "f", "", "tcpdump BPF filter expression (e.g. 'tcp port 80')")
	fs.StringVarP(&opts.OutputFile, "output", "o", "", "Output destination: path to .pcap file, '-' for stdout, or omit to launch Wireshark")
	fs.StringVarP((*string)(&opts.Mode), "mode", "m", string(capture.ModeAuto), "Capture mode: 'auto', 'ephemeral', or 'direct'")
	fs.StringVar(&opts.WiresharkPath, "wireshark-path", "", "Custom path to Wireshark binary")
	return fs
}

func parseCapArgs(rawArgs []string, defaults commonFlags) (*capture.Options, bool, error) {
	flags, podName, postPodArgs, showHelp, err := parsePrePodArgs(rawArgs, defaults, capUsage, nil)
	if err != nil || showHelp {
		return nil, showHelp, err
	}

	opts := capture.NewDefaultOptions()
	opts.PodName = podName
	opts.Namespace = flags.namespace
	opts.ContainerName = flags.containerName
	opts.DebugImage = flags.resolveDebugImage(k8s.DefaultDebugImage)
	opts.Kubeconfig = flags.kubeconfig
	opts.KubeContext = flags.kubeContext
	opts.Verbose = flags.verbose

	fs := newCapFlagSet(opts)
	if err := fs.Parse(postPodArgs); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return nil, true, nil
		}
		return nil, false, wrapPostPodFlagError("cap", podName, err)
	}

	if extra := fs.Args(); len(extra) > 0 {
		return nil, false, fmt.Errorf("unexpected positional arguments after POD_NAME: %s (Usage: %s)", strings.Join(extra, " "), capUsage)
	}

	return opts, false, nil
}

func init() {
	// Register command-specific flags on capCmd for help output.
	capCmd.Flags().StringP("interface", "i", "any", "Network interface to capture (e.g. eth0, any) (place after POD_NAME)")
	capCmd.Flags().StringP("filter", "f", "", "tcpdump BPF filter expression (e.g. 'tcp port 80') (place after POD_NAME)")
	capCmd.Flags().StringP("output", "o", "", "Output destination: path to .pcap file, '-' for stdout, or omit to launch Wireshark (place after POD_NAME)")
	capCmd.Flags().StringP("mode", "m", string(capture.ModeAuto), "Capture mode: 'auto', 'ephemeral', or 'direct' (place after POD_NAME)")
	capCmd.Flags().String("wireshark-path", "", "Custom path to Wireshark binary (place after POD_NAME)")

	RootCmd.AddCommand(capCmd)
}
