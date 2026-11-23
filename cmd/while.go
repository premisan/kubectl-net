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

const defaultWhileInterval = "1"

type whileOptions struct {
	podName       string
	interval      string
	namespace     string
	containerName string
	debugImage    string
	kubeconfig    string
	kubeContext   string
	verbose       bool
	commandArgs   []string
}

var whileCmd = &cobra.Command{
	Use:   "while [-i|--interval SECONDS] [POD_NAME] [COMMAND...]",
	Short: "Repeatedly execute a command at a fixed interval inside a target pod",
	Long: `Repeatedly execute an arbitrary command inside a Kubernetes pod's network namespace
using an Ephemeral Debug Container, running an infinite shell loop:

  while true; do <command>; echo; sleep <interval>; done

Flags for kubectl-net (such as -i/--interval, -n/--namespace, -c/--container) may be placed
before or after POD_NAME, as long as they appear before the command to execute.
Everything starting from the command token onward is passed verbatim to the debug container.`,
	Example: `  # Repeat curl every 1 second (default interval):
  kubectl net while my-pod curl -v https://ifconfig.me/

  # Repeat curl every 5 seconds (interval after or before pod name):
  kubectl net while my-pod --interval 5 curl -v https://ifconfig.me/
  kubectl net while -i 5 my-pod curl -v https://ifconfig.me/

  # Repeat curl with headers and custom namespace every 2 seconds:
  kubectl net while my-pod -n production -i 2 curl -s -I http://backend-svc:8080/healthz

  # Repeat a shell pipeline every 3 seconds:
  kubectl net while my-pod -i 3 "curl -s https://ifconfig.me/ && echo"`,
	DisableFlagParsing: true,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if shouldCompleteWhilePodName(args) {
			return k8s.CompletePods(toComplete, globalNamespace, globalKubeconfig, globalContext)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, rawArgs []string) error {
		defaults := whileOptions{
			interval:      defaultWhileInterval,
			namespace:     globalNamespace,
			containerName: globalContainer,
			debugImage:    globalDebugImage,
			kubeconfig:    globalKubeconfig,
			kubeContext:   globalContext,
			verbose:       globalVerbose,
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

		return clientCtx.RunInEphemeral(
			ctx,
			opts.podName,
			resolvedContainer,
			opts.debugImage,
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

	if len(rawArgs) == 0 {
		return opts, true, nil
	}

	i := 0
	// Phase 1: parse optional flags (-i/--interval, global flags) before POD_NAME, then POD_NAME itself.
	for i < len(rawArgs) {
		arg := rawArgs[i]

		if arg == "-h" || arg == "--help" {
			return opts, true, nil
		}

		if consumed, err := parseWhileIntervalFlag(rawArgs, i, &opts); err != nil {
			return opts, false, err
		} else if consumed > 0 {
			i += consumed
			continue
		}

		if consumed, err := parseWhileGlobalFlag(rawArgs, i, &opts); err != nil {
			return opts, false, err
		} else if consumed > 0 {
			i += consumed
			continue
		}

		if arg == "--" {
			return opts, false, fmt.Errorf("target pod name is required before '--'. Usage: kubectl net while [-i|--interval SECONDS] [POD_NAME] [COMMAND...]")
		}

		if strings.HasPrefix(arg, "-") {
			return opts, false, fmt.Errorf("unknown flag before pod name: %s", arg)
		}

		opts.podName = arg
		i++
		break
	}

	if opts.podName == "" {
		return opts, false, fmt.Errorf("target pod name is required. Usage: kubectl net while [-i|--interval SECONDS] [POD_NAME] [COMMAND...]")
	}

	// Phase 2: parse -i/--interval and optional global flags after POD_NAME until COMMAND begins.
	for i < len(rawArgs) {
		arg := rawArgs[i]

		if arg == "--" {
			opts.commandArgs = append([]string(nil), rawArgs[i+1:]...)
			break
		}

		if arg == "-h" || arg == "--help" {
			return opts, true, nil
		}

		if consumed, err := parseWhileIntervalFlag(rawArgs, i, &opts); err != nil {
			return opts, false, err
		} else if consumed > 0 {
			i += consumed
			continue
		}

		if consumed, err := parseWhileGlobalFlag(rawArgs, i, &opts); err != nil {
			return opts, false, err
		} else if consumed > 0 {
			i += consumed
			continue
		}

		if strings.HasPrefix(arg, "-") {
			return opts, false, fmt.Errorf("unknown flag '%s' before command (use '--' to separate flags from the command if needed)", arg)
		}

		// First non-flag token starts the command; pass everything from here verbatim.
		opts.commandArgs = append([]string(nil), rawArgs[i:]...)
		break
	}

	if len(opts.commandArgs) == 0 {
		return opts, false, fmt.Errorf("command to execute is required. Example: kubectl net while %s --interval 5 curl -v https://ifconfig.me/", opts.podName)
	}

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

func shouldCompleteWhilePodName(args []string) bool {
	i := 0
	for i < len(args) {
		arg := args[i]
		switch arg {
		case "-i", "--interval", "-n", "--namespace", "-c", "--container", "--image", "--kubeconfig", "--context":
			if i+1 >= len(args) {
				// Currently completing the value of this flag, not POD_NAME
				return false
			}
			i += 2
			continue
		case "-v", "--verbose":
			i++
			continue
		}
		if strings.HasPrefix(arg, "-") {
			i++
			continue
		}
		// Found a positional argument (POD_NAME already provided)
		return false
	}
	return true
}

func parseWhileGlobalFlag(rawArgs []string, i int, opts *whileOptions) (int, error) {
	arg := rawArgs[i]
	switch {
	case arg == "-n" || arg == "--namespace":
		if i+1 >= len(rawArgs) {
			return 0, fmt.Errorf("%s requires a value", arg)
		}
		opts.namespace = rawArgs[i+1]
		return 2, nil
	case strings.HasPrefix(arg, "--namespace="):
		opts.namespace = strings.TrimPrefix(arg, "--namespace=")
		return 1, nil
	case arg == "-c" || arg == "--container":
		if i+1 >= len(rawArgs) {
			return 0, fmt.Errorf("%s requires a value", arg)
		}
		opts.containerName = rawArgs[i+1]
		return 2, nil
	case strings.HasPrefix(arg, "--container="):
		opts.containerName = strings.TrimPrefix(arg, "--container=")
		return 1, nil
	case arg == "--image":
		if i+1 >= len(rawArgs) {
			return 0, fmt.Errorf("--image requires a value")
		}
		opts.debugImage = rawArgs[i+1]
		return 2, nil
	case strings.HasPrefix(arg, "--image="):
		opts.debugImage = strings.TrimPrefix(arg, "--image=")
		return 1, nil
	case arg == "--kubeconfig":
		if i+1 >= len(rawArgs) {
			return 0, fmt.Errorf("--kubeconfig requires a value")
		}
		opts.kubeconfig = rawArgs[i+1]
		return 2, nil
	case strings.HasPrefix(arg, "--kubeconfig="):
		opts.kubeconfig = strings.TrimPrefix(arg, "--kubeconfig=")
		return 1, nil
	case arg == "--context":
		if i+1 >= len(rawArgs) {
			return 0, fmt.Errorf("--context requires a value")
		}
		opts.kubeContext = rawArgs[i+1]
		return 2, nil
	case strings.HasPrefix(arg, "--context="):
		opts.kubeContext = strings.TrimPrefix(arg, "--context=")
		return 1, nil
	case arg == "-v" || arg == "--verbose":
		opts.verbose = true
		return 1, nil
	default:
		return 0, nil
	}
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
	// (e.g. kubectl net while my-pod --interval 5 "curl -s https://ifconfig.me/ && echo"),
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
	whileCmd.Flags().StringP("interval", "i", defaultWhileInterval, "Interval between command executions in seconds")
	RootCmd.AddCommand(whileCmd)
}
