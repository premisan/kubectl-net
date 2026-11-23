package cmd

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/premisan/kubectl-net/pkg/k8s"
)

const (
	defaultWhileInterval = "1"
	whileUsage           = "kubectl net while [GLOBAL_FLAGS...] [-i|--interval SECONDS] [POD_NAME] [COMMAND...]"
)

type whileOptions struct {
	podName       string
	interval      string
	namespace     string
	containerName string
	debugImage    string
	imageExplicit bool
	kubeconfig    string
	kubeContext   string
	verbose       bool
	commandArgs   []string
}

var whileCmd = &cobra.Command{
	Use:   "while [GLOBAL_FLAGS...] [-i|--interval SECONDS] [POD_NAME] [COMMAND...]",
	Short: "Repeatedly execute a command at a fixed interval inside a target pod",
	Long: `Repeatedly execute an arbitrary command inside a Kubernetes pod's network namespace
using an Ephemeral Debug Container, running an infinite shell loop:

  while true; do <command>; echo; sleep <interval>; done

All kubectl-net flags (-i/--interval, -n/--namespace, -c/--container, --image,
--context, --kubeconfig, -v/--verbose) must be placed BEFORE POD_NAME.
Everything AFTER POD_NAME is passed verbatim as the command to repeat.`,
	Example: `  # Repeat curl every 1 second (default interval):
  kubectl net while my-pod curl -v https://ifconfig.me/

  # Repeat curl every 5 seconds (--interval or -i before POD_NAME):
  kubectl net while --interval 5 my-pod curl -v https://ifconfig.me/
  kubectl net while -i 5 my-pod curl -v https://ifconfig.me/

  # Target a specific namespace and repeat every 2 seconds:
  kubectl net while -n production -i 2 my-pod curl -s -I http://backend-svc:8080/healthz

  # Repeat a shell pipeline every 3 seconds:
  kubectl net while -i 3 my-pod "curl -s https://ifconfig.me/ && echo"`,
	DisableFlagParsing: true,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completePrePodArgs(args, toComplete, map[string]bool{
			"-i":         true,
			"--interval": true,
		})
	},
	RunE: func(cmd *cobra.Command, rawArgs []string) error {
		cf := defaultCommonFlags()
		defaults := whileOptions{
			interval:      defaultWhileInterval,
			namespace:     cf.namespace,
			containerName: cf.containerName,
			debugImage:    cf.debugImage,
			imageExplicit: cf.imageExplicit,
			kubeconfig:    cf.kubeconfig,
			kubeContext:   cf.kubeContext,
			verbose:       cf.verbose,
		}

		opts, showHelp, err := parseWhileArgs(rawArgs, defaults)
		if err != nil {
			return err
		}
		if showHelp {
			return cmd.Help()
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		clientCtx, err := k8s.NewClientContext(opts.kubeconfig, opts.kubeContext, opts.namespace)
		if err != nil {
			return fmt.Errorf("failed to configure kubernetes client: %w", err)
		}

		pod, err := clientCtx.GetPod(ctx, opts.podName)
		if err != nil {
			return err
		}

		resolvedContainer, err := k8s.FindTargetContainer(pod, opts.containerName)
		if err != nil {
			return err
		}

		whileExecCmd := buildWhileCommand(opts.interval, opts.commandArgs)

		fmt.Fprintf(os.Stderr, "==> Executing loop in pod '%s' (namespace: %s, container: %s, interval: %ss): %s\n",
			opts.podName, clientCtx.Namespace, resolvedContainer, opts.interval, strings.Join(opts.commandArgs, " "))
		fmt.Fprintf(os.Stderr, "==> Press Ctrl+C to stop.\n")
		if opts.verbose {
			fmt.Fprintf(os.Stderr, "==> Debug command: %s\n", strings.Join(whileExecCmd, " "))
		}

		// Use TTY when attached to an interactive terminal so Ctrl+C sends SIGINT
		// directly to the remote shell process group and avoids orphaned loops.
		useTTY := term.IsTerminal(int(os.Stdin.Fd())) &&
			term.IsTerminal(int(os.Stdout.Fd())) &&
			term.IsTerminal(int(os.Stderr.Fd()))

		var stderrWriter io.Writer = os.Stderr
		if useTTY {
			stderrWriter = nil
		}

		debugImage := k8s.DefaultDebugImage
		if opts.imageExplicit && opts.debugImage != "" {
			debugImage = opts.debugImage
		} else if opts.debugImage != "" {
			debugImage = opts.debugImage
		}

		return clientCtx.RunInEphemeral(
			ctx,
			opts.podName,
			resolvedContainer,
			debugImage,
			whileExecCmd,
			os.Stdin,
			os.Stdout,
			stderrWriter,
			useTTY,
			os.Stderr,
		)
	},
}

func parseWhileArgs(rawArgs []string, defaults whileOptions) (whileOptions, bool, error) {
	opts := defaults
	if opts.interval == "" {
		opts.interval = defaultWhileInterval
	}

	cfDefaults := commonFlags{
		namespace:     opts.namespace,
		containerName: opts.containerName,
		debugImage:    opts.debugImage,
		imageExplicit: opts.imageExplicit,
		kubeconfig:    opts.kubeconfig,
		kubeContext:   opts.kubeContext,
		verbose:       opts.verbose,
	}

	flags, podName, postPodArgs, showHelp, err := parsePrePodArgs(
		rawArgs,
		cfDefaults,
		whileUsage,
		func(args []string, i int) (int, error) {
			return parseWhileIntervalFlag(args, i, &opts)
		},
	)
	if err != nil || showHelp {
		return opts, showHelp, err
	}

	opts.podName = podName
	opts.namespace = flags.namespace
	opts.containerName = flags.containerName
	opts.debugImage = flags.debugImage
	opts.imageExplicit = flags.imageExplicit
	opts.kubeconfig = flags.kubeconfig
	opts.kubeContext = flags.kubeContext
	opts.verbose = flags.verbose

	if len(postPodArgs) == 0 {
		return opts, false, fmt.Errorf("command to execute is required. Example: kubectl net while -i 5 %s curl -v https://ifconfig.me/", opts.podName)
	}

	if postPodArgs[0] == "--" {
		postPodArgs = postPodArgs[1:]
		if len(postPodArgs) == 0 {
			return opts, false, fmt.Errorf("command to execute is required after '--'. Example: kubectl net while -i 5 %s -- curl -v https://ifconfig.me/", opts.podName)
		}
	} else if strings.HasPrefix(postPodArgs[0], "-") {
		return opts, false, fmt.Errorf("flag %q appeared after POD_NAME; kubectl-net flags (such as -i/--interval, -n/--namespace) must be placed before POD_NAME (e.g. kubectl net while -i 5 %s curl ...)", postPodArgs[0], opts.podName)
	}

	opts.commandArgs = postPodArgs
	return opts, false, nil
}

func parseWhileIntervalFlag(rawArgs []string, i int, opts *whileOptions) (int, error) {
	arg := rawArgs[i]
	if arg == "--interval" || arg == "-i" {
		if i+1 >= len(rawArgs) {
			return 0, fmt.Errorf("%s requires a value in seconds (e.g. %s 5)", arg, arg)
		}
		val, err := validateInterval(rawArgs[i+1])
		if err != nil {
			return 0, err
		}
		opts.interval = val
		return 2, nil
	}
	if strings.HasPrefix(arg, "--interval=") || strings.HasPrefix(arg, "-i=") {
		rawVal := strings.TrimPrefix(arg, "--interval=")
		if strings.HasPrefix(arg, "-i=") {
			rawVal = strings.TrimPrefix(arg, "-i=")
		}
		val, err := validateInterval(rawVal)
		if err != nil {
			return 0, err
		}
		opts.interval = val
		return 1, nil
	}
	return 0, nil
}

func validateInterval(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("invalid interval value %q: must be a positive number (seconds)", raw)
	}
	val, err := strconv.ParseFloat(trimmed, 64)
	if err != nil || math.IsNaN(val) || math.IsInf(val, 0) || val <= 0 {
		return "", fmt.Errorf("invalid interval value %q: must be a positive number (seconds)", raw)
	}
	return strconv.FormatFloat(val, 'f', -1, 64), nil
}

func buildWhileCommand(interval string, cmdArgs []string) []string {
	// If the user passed a single shell command string containing whitespace or shell operators
	// (e.g. kubectl net while -i 5 my-pod "curl -s https://ifconfig.me/ && echo"),
	// execute it directly inside the shell loop.
	if len(cmdArgs) == 1 && strings.ContainsAny(cmdArgs[0], " \t\n|&;<>()$`\"'") {
		script := fmt.Sprintf("trap 'exit 130' INT TERM; while true; do %s; echo; sleep %s; done", cmdArgs[0], interval)
		return []string{"sh", "-c", script}
	}

	// Preserve exact argument boundaries (spaces, quotes, special characters) via positional parameters "$@".
	script := fmt.Sprintf(`trap 'exit 130' INT TERM; while true; do "$@"; echo; sleep %s; done`, interval)
	return append([]string{"sh", "-c", script, "_"}, cmdArgs...)
}

func init() {
	whileCmd.Flags().StringP("interval", "i", defaultWhileInterval, "Interval between command executions in seconds (place before POD_NAME)")
	RootCmd.AddCommand(whileCmd)
}
