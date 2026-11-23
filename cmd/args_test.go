package cmd

import (
	"reflect"
	"strings"
	"testing"

	"github.com/premisan/kubectl-net/pkg/capture"
)

func TestUnifiedFlagPositioning(t *testing.T) {
	defaults := commonFlags{
		debugImage: "nicolaka/netshoot:latest",
	}

	t.Run("curl passes -h, -n, -c, -v verbatim after POD_NAME", func(t *testing.T) {
		opts, showHelp, err := parseCurlArgs(
			[]string{"-n", "prod", "-c", "app", "my-pod", "-v", "-c", "/tmp/cookies", "-n", "-h", "https://example.com"},
			defaults,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if showHelp {
			t.Fatalf("did not expect showHelp")
		}
		if opts.flags.namespace != "prod" || opts.flags.containerName != "app" || opts.podName != "my-pod" {
			t.Fatalf("unexpected pre-pod parsing: %+v", opts)
		}
		wantCurl := []string{"-v", "-c", "/tmp/cookies", "-n", "-h", "https://example.com"}
		if !reflect.DeepEqual(opts.curlArgs, wantCurl) {
			t.Fatalf("curlArgs = %#v, want %#v", opts.curlArgs, wantCurl)
		}
	})

	t.Run("ping accepts global flags before POD_NAME and ping options after POD_NAME", func(t *testing.T) {
		opts, showHelp, err := parsePingArgs([]string{"-n", "prod", "pod-a", "8.8.8.8", "-C", "10", "-t", "3"}, defaults)
		if err != nil || showHelp {
			t.Fatalf("unexpected result: showHelp=%v, err=%v", showHelp, err)
		}
		if opts.flags.namespace != "prod" || opts.podName != "pod-a" || opts.targetHost != "8.8.8.8" || opts.count != 10 || opts.timeout != 3 {
			t.Fatalf("unexpected ping opts: %+v", opts)
		}
	})

	t.Run("ping rejects -n after POD_NAME with guidance", func(t *testing.T) {
		_, _, err := parsePingArgs([]string{"pod-a", "8.8.8.8", "-n", "prod"}, defaults)
		if err == nil || !strings.Contains(err.Error(), "must be placed before POD_NAME") {
			t.Fatalf("expected before POD_NAME guidance error, got: %v", err)
		}
	})

	t.Run("dig accepts global flags before POD_NAME and dig options after POD_NAME", func(t *testing.T) {
		opts, showHelp, err := parseDigArgs([]string{"-n", "prod", "pod-a", "google.com", "-t", "AAAA", "-s", "8.8.8.8"}, defaults)
		if err != nil || showHelp {
			t.Fatalf("unexpected result: showHelp=%v, err=%v", showHelp, err)
		}
		if opts.flags.namespace != "prod" || opts.podName != "pod-a" || opts.domainName != "google.com" || opts.recordType != "AAAA" || opts.dnsServer != "8.8.8.8" {
			t.Fatalf("unexpected dig opts: %+v", opts)
		}
	})

	t.Run("dig rejects -n after POD_NAME", func(t *testing.T) {
		_, _, err := parseDigArgs([]string{"pod-a", "google.com", "-n", "prod"}, defaults)
		if err == nil || !strings.Contains(err.Error(), "must be placed before POD_NAME") {
			t.Fatalf("expected before POD_NAME guidance error, got: %v", err)
		}
	})

	t.Run("nc accepts global flags before POD_NAME and nc options after POD_NAME", func(t *testing.T) {
		opts, showHelp, err := parseNcArgs([]string{"-n", "prod", "pod-a", "10.0.0.1", "3306", "-z", "-w", "3"}, defaults)
		if err != nil || showHelp {
			t.Fatalf("unexpected result: showHelp=%v, err=%v", showHelp, err)
		}
		if opts.flags.namespace != "prod" || opts.podName != "pod-a" || opts.targetHost != "10.0.0.1" || opts.portStr != "3306" || !opts.scan || opts.timeout != 3 {
			t.Fatalf("unexpected nc opts: %+v", opts)
		}
	})

	t.Run("nc rejects -n after POD_NAME", func(t *testing.T) {
		_, _, err := parseNcArgs([]string{"pod-a", "10.0.0.1", "3306", "-n", "prod"}, defaults)
		if err == nil || !strings.Contains(err.Error(), "must be placed before POD_NAME") {
			t.Fatalf("expected before POD_NAME guidance error, got: %v", err)
		}
	})

	t.Run("cap accepts global flags before POD_NAME and capture options after POD_NAME", func(t *testing.T) {
		opts, showHelp, err := parseCapArgs([]string{"-n", "prod", "-c", "app", "pod-a", "-i", "eth0", "-f", "port 80", "-o", "./out.pcap", "-m", "ephemeral"}, defaults)
		if err != nil || showHelp {
			t.Fatalf("unexpected result: showHelp=%v, err=%v", showHelp, err)
		}
		if opts.Namespace != "prod" || opts.ContainerName != "app" || opts.PodName != "pod-a" ||
			opts.Interface != "eth0" || opts.Filter != "port 80" || opts.OutputFile != "./out.pcap" || opts.Mode != capture.ModeEphemeral {
			t.Fatalf("unexpected cap opts: %+v", opts)
		}
	})

	t.Run("cap rejects -n after POD_NAME", func(t *testing.T) {
		_, _, err := parseCapArgs([]string{"pod-a", "-n", "prod", "-f", "port 80"}, defaults)
		if err == nil || !strings.Contains(err.Error(), "must be placed before POD_NAME") {
			t.Fatalf("expected before POD_NAME guidance error, got: %v", err)
		}
	})

	t.Run("sh accepts global flags before POD_NAME and rejects them after POD_NAME", func(t *testing.T) {
		opts, showHelp, err := parseShArgs([]string{"-n", "prod", "-c", "app", "pod-a"}, defaults)
		if err != nil || showHelp {
			t.Fatalf("unexpected result: showHelp=%v, err=%v", showHelp, err)
		}
		if opts.flags.namespace != "prod" || opts.flags.containerName != "app" || opts.podName != "pod-a" {
			t.Fatalf("unexpected sh opts: %+v", opts)
		}

		_, _, err = parseShArgs([]string{"pod-a", "-n", "prod"}, defaults)
		if err == nil || !strings.Contains(err.Error(), "must be placed before POD_NAME") {
			t.Fatalf("expected before POD_NAME guidance error, got: %v", err)
		}
	})
}
