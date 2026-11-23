package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/premisan/kubectl-net/pkg/k8s"
)

// commonFlags holds kubectl-net global flags parsed before POD_NAME.
type commonFlags struct {
	namespace     string
	containerName string
	debugImage    string
	imageExplicit bool
	kubeconfig    string
	kubeContext   string
	verbose       bool
}

func defaultCommonFlags() commonFlags {
	return commonFlags{
		namespace:     globalNamespace,
		containerName: globalContainer,
		debugImage:    globalDebugImage,
		imageExplicit: RootCmd.PersistentFlags().Changed("image"),
		kubeconfig:    globalKubeconfig,
		kubeContext:   globalContext,
		verbose:       globalVerbose,
	}
}

func (f commonFlags) resolveDebugImage(defaultImage string) string {
	if f.imageExplicit && f.debugImage != "" {
		return f.debugImage
	}
	if defaultImage != "" {
		return defaultImage
	}
	if f.debugImage != "" {
		return f.debugImage
	}
	return k8s.DefaultDebugImage
}

// prePodFlagHandler handles subcommand-specific kubectl-net flags that appear before POD_NAME.
// It returns the number of consumed arguments (0 if the token is not handled).
type prePodFlagHandler func(rawArgs []string, i int) (int, error)

// parsePrePodArgs enforces the unified CLI contract:
//
//	kubectl net <subcommand> [KUBECTL_NET_FLAGS...] <POD_NAME> [COMMAND_ARGS...]
//
// All kubectl-net flags must appear before POD_NAME; everything after POD_NAME is returned verbatim
// as postPodArgs for the target command.
func parsePrePodArgs(
	rawArgs []string,
	defaults commonFlags,
	usage string,
	extraHandler prePodFlagHandler,
) (commonFlags, string, []string, bool, error) {
	flags := defaults

	if len(rawArgs) == 0 {
		return flags, "", nil, true, nil
	}

	i := 0
	for i < len(rawArgs) {
		arg := rawArgs[i]

		if arg == "-h" || arg == "--help" {
			return flags, "", nil, true, nil
		}

		if extraHandler != nil {
			consumed, err := extraHandler(rawArgs, i)
			if err != nil {
				return flags, "", nil, false, err
			}
			if consumed > 0 {
				i += consumed
				continue
			}
		}

		consumed, err := parseCommonGlobalFlag(rawArgs, i, &flags)
		if err != nil {
			return flags, "", nil, false, err
		}
		if consumed > 0 {
			i += consumed
			continue
		}

		if arg == "--" {
			return flags, "", nil, false, fmt.Errorf("target pod name is required before '--'. Usage: %s", usage)
		}

		if strings.HasPrefix(arg, "-") {
			return flags, "", nil, false, fmt.Errorf("unknown flag %q before POD_NAME (command-specific options must be placed after POD_NAME)", arg)
		}

		podName := arg
		postPodArgs := append([]string(nil), rawArgs[i+1:]...)
		return flags, podName, postPodArgs, false, nil
	}

	return flags, "", nil, false, fmt.Errorf("target pod name is required. Usage: %s", usage)
}

func parseCommonGlobalFlag(rawArgs []string, i int, flags *commonFlags) (int, error) {
	arg := rawArgs[i]
	switch {
	case arg == "-n" || arg == "--namespace":
		if i+1 >= len(rawArgs) {
			return 0, fmt.Errorf("%s requires a value", arg)
		}
		flags.namespace = rawArgs[i+1]
		return 2, nil
	case strings.HasPrefix(arg, "--namespace="):
		flags.namespace = strings.TrimPrefix(arg, "--namespace=")
		return 1, nil
	case strings.HasPrefix(arg, "-n="):
		flags.namespace = strings.TrimPrefix(arg, "-n=")
		return 1, nil
	case arg == "-c" || arg == "--container":
		if i+1 >= len(rawArgs) {
			return 0, fmt.Errorf("%s requires a value", arg)
		}
		flags.containerName = rawArgs[i+1]
		return 2, nil
	case strings.HasPrefix(arg, "--container="):
		flags.containerName = strings.TrimPrefix(arg, "--container=")
		return 1, nil
	case strings.HasPrefix(arg, "-c="):
		flags.containerName = strings.TrimPrefix(arg, "-c=")
		return 1, nil
	case arg == "--image":
		if i+1 >= len(rawArgs) {
			return 0, fmt.Errorf("--image requires a value")
		}
		flags.debugImage = rawArgs[i+1]
		flags.imageExplicit = true
		return 2, nil
	case strings.HasPrefix(arg, "--image="):
		flags.debugImage = strings.TrimPrefix(arg, "--image=")
		flags.imageExplicit = true
		return 1, nil
	case arg == "--kubeconfig":
		if i+1 >= len(rawArgs) {
			return 0, fmt.Errorf("--kubeconfig requires a value")
		}
		flags.kubeconfig = rawArgs[i+1]
		return 2, nil
	case strings.HasPrefix(arg, "--kubeconfig="):
		flags.kubeconfig = strings.TrimPrefix(arg, "--kubeconfig=")
		return 1, nil
	case arg == "--context":
		if i+1 >= len(rawArgs) {
			return 0, fmt.Errorf("--context requires a value")
		}
		flags.kubeContext = rawArgs[i+1]
		return 2, nil
	case strings.HasPrefix(arg, "--context="):
		flags.kubeContext = strings.TrimPrefix(arg, "--context=")
		return 1, nil
	case arg == "-v" || arg == "--verbose":
		flags.verbose = true
		return 1, nil
	default:
		return 0, nil
	}
}

// completePrePodArgs provides shell completion for kubectl-net flags and POD_NAME
// when DisableFlagParsing is enabled on a subcommand.
func completePrePodArgs(
	args []string,
	toComplete string,
	extraValueFlags map[string]bool,
) ([]string, cobra.ShellCompDirective) {
	ns := globalNamespace
	kubeconfig := globalKubeconfig
	kubeContext := globalContext

	i := 0
	for i < len(args) {
		arg := args[i]

		switch arg {
		case "-n", "--namespace":
			if i+1 >= len(args) {
				return k8s.CompleteNamespaces(toComplete, kubeconfig, kubeContext)
			}
			ns = args[i+1]
			i += 2
			continue
		case "--kubeconfig":
			if i+1 >= len(args) {
				return nil, cobra.ShellCompDirectiveDefault
			}
			kubeconfig = args[i+1]
			i += 2
			continue
		case "--context":
			if i+1 >= len(args) {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			kubeContext = args[i+1]
			i += 2
			continue
		case "-c", "--container", "--image":
			if i+1 >= len(args) {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			i += 2
			continue
		case "-v", "--verbose":
			i++
			continue
		}

		if extraValueFlags != nil && extraValueFlags[arg] {
			if i+1 >= len(args) {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			i += 2
			continue
		}

		if strings.HasPrefix(arg, "--namespace=") {
			ns = strings.TrimPrefix(arg, "--namespace=")
			i++
			continue
		}
		if strings.HasPrefix(arg, "-n=") {
			ns = strings.TrimPrefix(arg, "-n=")
			i++
			continue
		}
		if strings.HasPrefix(arg, "--kubeconfig=") {
			kubeconfig = strings.TrimPrefix(arg, "--kubeconfig=")
			i++
			continue
		}
		if strings.HasPrefix(arg, "--context=") {
			kubeContext = strings.TrimPrefix(arg, "--context=")
			i++
			continue
		}
		if strings.HasPrefix(arg, "-") {
			i++
			continue
		}

		// POD_NAME has already been provided.
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	if strings.HasPrefix(toComplete, "-") {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return k8s.CompletePods(toComplete, ns, kubeconfig, kubeContext)
}

func isGlobalFlagToken(arg string) bool {
	switch arg {
	case "-n", "--namespace", "-c", "--container", "--image", "--kubeconfig", "--context", "-v", "--verbose":
		return true
	}
	for _, prefix := range []string{"--namespace=", "-n=", "--container=", "-c=", "--image=", "--kubeconfig=", "--context="} {
		if strings.HasPrefix(arg, prefix) {
			return true
		}
	}
	return false
}

func wrapPostPodFlagError(subcmd, podName string, err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	for _, hint := range []string{
		"unknown flag: --namespace",
		"unknown shorthand flag: 'n'",
		"unknown flag: --container",
		"unknown shorthand flag: 'c'",
		"unknown flag: --image",
		"unknown flag: --kubeconfig",
		"unknown flag: --context",
		"unknown flag: --verbose",
		"unknown shorthand flag: 'v'",
	} {
		if strings.Contains(msg, hint) {
			return fmt.Errorf("%w (kubectl-net flags such as -n/--namespace, -c/--container, --image must be placed before POD_NAME, e.g. kubectl net %s -n <namespace> %s ...)", err, subcmd, podName)
		}
	}
	return err
}
