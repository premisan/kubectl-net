package cmd

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/premisan/kubectl-net/pkg/k8s"
)

type dbClientSpec struct {
	name            string
	binary          string
	defaultImage    string
	containerPrefix string
	use             string
	aliases         []string
	short           string
	long            string
	example         string
}

type dbClientOptions struct {
	flags      commonFlags
	podName    string
	debugImage string
	clientArgs []string
}

func newDBClientCommand(spec dbClientSpec) *cobra.Command {
	usage := fmt.Sprintf("kubectl net %s", spec.use)
	return &cobra.Command{
		Use:                spec.use,
		Aliases:            spec.aliases,
		Short:              spec.short,
		Long:               spec.long,
		Example:            spec.example,
		DisableFlagParsing: true,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completePrePodArgs(args, toComplete, nil)
		},
		RunE: func(cmd *cobra.Command, rawArgs []string) error {
			opts, showHelp, err := parseDBClientArgs(rawArgs, defaultCommonFlags(), spec.defaultImage, usage)
			if err != nil {
				return err
			}
			if showHelp {
				return cmd.Help()
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

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

			execCmd := append([]string{spec.binary}, opts.clientArgs...)
			redacted := redactSensitiveArgs(spec.binary, opts.clientArgs)
			displayCmd := append([]string{spec.binary}, redacted...)

			fmt.Fprintf(os.Stderr, "==> Executing in pod '%s' (namespace: %s, container: %s, image: %s): %s\n",
				opts.podName, clientCtx.Namespace, resolvedContainer, opts.debugImage, strings.Join(displayCmd, " "))

			useTTY := term.IsTerminal(int(os.Stdin.Fd())) &&
				term.IsTerminal(int(os.Stdout.Fd()))

			var stderrWriter io.Writer = os.Stderr
			if useTTY {
				stderrWriter = nil
			}

			return clientCtx.RunInEphemeralWithOptions(
				ctx,
				k8s.EphemeralOptions{
					PodName:         opts.podName,
					TargetContainer: resolvedContainer,
					DebugImage:      opts.debugImage,
					NamePrefix:      spec.containerPrefix,
					Privileged:      false,
					WaitTimeout:     k8s.DefaultEphemeralWaitTimeout,
					LogWriter:       os.Stderr,
				},
				execCmd,
				os.Stdin,
				os.Stdout,
				stderrWriter,
				useTTY,
			)
		},
	}
}

func parseDBClientArgs(
	rawArgs []string,
	defaults commonFlags,
	defaultImage string,
	usage string,
) (dbClientOptions, bool, error) {
	flags, podName, postPodArgs, showHelp, err := parsePrePodArgs(rawArgs, defaults, usage, nil)
	if err != nil || showHelp {
		return dbClientOptions{}, showHelp, err
	}

	return dbClientOptions{
		flags:      flags,
		podName:    podName,
		debugImage: flags.resolveDebugImage(defaultImage),
		clientArgs: postPodArgs,
	}, false, nil
}

var kvPasswordPattern = regexp.MustCompile(`(?i)\b(password=)('[^']*'|"[^"]*"|\S+)`)

// redactSensitiveArgs masks inline credentials before logging the remote command to stderr.
func redactSensitiveArgs(binary string, args []string) []string {
	out := make([]string, len(args))
	copy(out, args)

	for i := 0; i < len(out); i++ {
		arg := out[i]

		switch binary {
		case "mysql":
			if len(arg) > 2 && strings.HasPrefix(arg, "-p") {
				out[i] = "-p***"
				continue
			}
			if strings.HasPrefix(arg, "--password=") {
				out[i] = "--password=***"
				continue
			}
		case "redis-cli":
			if (arg == "-a" || arg == "--pass") && i+1 < len(out) {
				out[i+1] = "***"
				i++
				continue
			}
			if len(arg) > 2 && strings.HasPrefix(arg, "-a") {
				out[i] = "-a***"
				continue
			}
			if strings.HasPrefix(arg, "--pass=") {
				out[i] = "--pass=***"
				continue
			}
		case "psql":
			if kvPasswordPattern.MatchString(arg) {
				out[i] = kvPasswordPattern.ReplaceAllString(arg, "${1}***")
				continue
			}
		}

		if redactedURI, ok := redactURI(arg); ok {
			out[i] = redactedURI
		}
	}

	return out
}

func redactURI(raw string) (string, bool) {
	lower := strings.ToLower(raw)
	if !strings.Contains(lower, "://") {
		return raw, false
	}
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw, false
	}
	if _, hasPass := u.User.Password(); !hasPass {
		return raw, false
	}
	// Preserve original structure while masking only the password part in userinfo.
	schemeSep := strings.Index(raw, "://")
	if schemeSep < 0 {
		return raw, false
	}
	rest := raw[schemeSep+3:]
	atIdx := strings.Index(rest, "@")
	if atIdx < 0 {
		return raw, false
	}
	userInfo := rest[:atIdx]
	colonIdx := strings.Index(userInfo, ":")
	if colonIdx < 0 {
		return raw, false
	}
	masked := raw[:schemeSep+3] + userInfo[:colonIdx+1] + "***" + rest[atIdx:]
	return masked, true
}
