package cmd

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestParseTelnetArgs(t *testing.T) {
	defaults := commonFlags{
		debugImage: "nicolaka/netshoot:latest",
	}

	tests := []struct {
		name        string
		rawArgs     []string
		wantPod     string
		wantNS      string
		wantImage   string
		wantArgs    []string
		wantHelp    bool
		expectError bool
		errContains string
	}{
		{
			name:      "basic host and port",
			rawArgs:   []string{"my-pod", "backend-service", "8080"},
			wantPod:   "my-pod",
			wantImage: "nicolaka/netshoot:latest",
			wantArgs:  []string{"backend-service", "8080"},
		},
		{
			name:      "global flags before POD_NAME and telnet options after POD_NAME",
			rawArgs:   []string{"-n", "prod", "-c", "app", "--image", "custom/netshoot:latest", "my-pod", "-l", "user", "10.0.0.1", "23"},
			wantPod:   "my-pod",
			wantNS:    "prod",
			wantImage: "custom/netshoot:latest",
			wantArgs:  []string{"-l", "user", "10.0.0.1", "23"},
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
			name:        "missing host after POD_NAME returns error",
			rawArgs:     []string{"my-pod"},
			expectError: true,
			errContains: "target host",
		},
		{
			name:        "unknown flag before POD_NAME is rejected",
			rawArgs:     []string{"-l", "user", "my-pod", "10.0.0.1", "23"},
			expectError: true,
			errContains: "before POD_NAME",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, showHelp, err := parseTelnetArgs(tt.rawArgs, defaults)
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
			if img := got.flags.resolveDebugImage("nicolaka/netshoot:latest"); img != tt.wantImage {
				t.Fatalf("resolved image = %q, want %q", img, tt.wantImage)
			}
			if !reflect.DeepEqual(got.telnetArgs, tt.wantArgs) {
				t.Fatalf("telnetArgs = %#v, want %#v", got.telnetArgs, tt.wantArgs)
			}
		})
	}
}

func TestTelnetCmdHelp(t *testing.T) {
	buf := new(bytes.Buffer)
	RootCmd.SetOut(buf)
	RootCmd.SetErr(buf)
	RootCmd.SetArgs([]string{"telnet", "--help"})

	if err := RootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error executing telnet --help: %v", err)
	}
	if !strings.Contains(buf.String(), "telnet") {
		t.Fatalf("expected telnet in help output, got: %s", buf.String())
	}
}
