package cmd

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestParseNetstatArgs(t *testing.T) {
	defaults := commonFlags{
		debugImage: "nicolaka/netshoot:latest",
	}

	tests := []struct {
		name          string
		rawArgs       []string
		wantPod       string
		wantNS        string
		wantContainer string
		wantImage     string
		wantArgs      []string
		wantHelp      bool
		expectError   bool
		errContains   string
	}{
		{
			name:      "pod name only runs default netstat",
			rawArgs:   []string{"my-pod"},
			wantPod:   "my-pod",
			wantImage: "nicolaka/netshoot:latest",
			wantArgs:  nil,
		},
		{
			name:      "listening ports with -tulnp",
			rawArgs:   []string{"my-pod", "-tulnp"},
			wantPod:   "my-pod",
			wantImage: "nicolaka/netshoot:latest",
			wantArgs:  []string{"-tulnp"},
		},
		{
			name:          "global flags before POD_NAME and conflicting netstat flags after POD_NAME",
			rawArgs:       []string{"-n", "prod", "-c", "app", "--image", "custom/netshoot:latest", "my-pod", "-antp", "-n", "-c", "-v"},
			wantPod:       "my-pod",
			wantNS:        "prod",
			wantContainer: "app",
			wantImage:     "custom/netshoot:latest",
			wantArgs:      []string{"-antp", "-n", "-c", "-v"},
		},
		{
			name:      "-h after POD_NAME is passed verbatim to netstat",
			rawArgs:   []string{"my-pod", "-h"},
			wantPod:   "my-pod",
			wantImage: "nicolaka/netshoot:latest",
			wantArgs:  []string{"-h"},
		},
		{
			name:     "no args shows help",
			rawArgs:  []string{},
			wantHelp: true,
		},
		{
			name:     "-h before POD_NAME shows help",
			rawArgs:  []string{"-h"},
			wantHelp: true,
		},
		{
			name:        "netstat flag before POD_NAME is rejected",
			rawArgs:     []string{"-tulnp", "my-pod"},
			expectError: true,
			errContains: "before POD_NAME",
		},
		{
			name:        "missing POD_NAME after global flags returns error",
			rawArgs:     []string{"-n", "prod"},
			expectError: true,
			errContains: "target pod name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, showHelp, err := parseNetstatArgs(tt.rawArgs, defaults)
			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContains)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if showHelp != tt.wantHelp {
				t.Fatalf("showHelp = %v, want %v", showHelp, tt.wantHelp)
			}
			if tt.wantHelp {
				return
			}
			if got.podName != tt.wantPod {
				t.Fatalf("podName = %q, want %q", got.podName, tt.wantPod)
			}
			if got.flags.namespace != tt.wantNS {
				t.Fatalf("namespace = %q, want %q", got.flags.namespace, tt.wantNS)
			}
			if got.flags.containerName != tt.wantContainer {
				t.Fatalf("containerName = %q, want %q", got.flags.containerName, tt.wantContainer)
			}
			if img := got.flags.resolveDebugImage("nicolaka/netshoot:latest"); img != tt.wantImage {
				t.Fatalf("resolved image = %q, want %q", img, tt.wantImage)
			}
			if !reflect.DeepEqual(got.netstatArgs, tt.wantArgs) {
				t.Fatalf("netstatArgs = %#v, want %#v", got.netstatArgs, tt.wantArgs)
			}
		})
	}
}

func TestNetstatCmdHelp(t *testing.T) {
	buf := new(bytes.Buffer)
	RootCmd.SetOut(buf)
	RootCmd.SetErr(buf)
	RootCmd.SetArgs([]string{"netstat", "--help"})

	if err := RootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error executing netstat --help: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "netstat") || !strings.Contains(out, "-tulnp") {
		t.Fatalf("expected netstat usage and examples in help output, got: %s", out)
	}
}
