package cmd

import (
	"bytes"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestParseWhileArgs(t *testing.T) {
	baseDefaults := whileOptions{
		interval:   defaultWhileInterval,
		debugImage: "nicolaka/netshoot:latest",
	}

	tests := []struct {
		name        string
		rawArgs     []string
		wantOpts    whileOptions
		wantHelp    bool
		expectError bool
		errContains string
	}{
		{
			name:     "no args shows help",
			rawArgs:  []string{},
			wantHelp: true,
		},
		{
			name:     "help flag before pod name",
			rawArgs:  []string{"--help"},
			wantHelp: true,
		},
		{
			name:     "short help flag before pod name",
			rawArgs:  []string{"-h"},
			wantHelp: true,
		},
		{
			name:    "default interval 1s with curl args",
			rawArgs: []string{"my-pod", "curl", "-v", "https://ifconfig.me/"},
			wantOpts: whileOptions{
				podName:     "my-pod",
				interval:    "1",
				debugImage:  "nicolaka/netshoot:latest",
				commandArgs: []string{"curl", "-v", "https://ifconfig.me/"},
			},
		},
		{
			name:    "custom interval before pod name",
			rawArgs: []string{"--interval", "5", "my-pod", "curl", "-v", "https://ifconfig.me/"},
			wantOpts: whileOptions{
				podName:     "my-pod",
				interval:    "5",
				debugImage:  "nicolaka/netshoot:latest",
				commandArgs: []string{"curl", "-v", "https://ifconfig.me/"},
			},
		},
		{
			name:    "custom interval with equals syntax before pod name",
			rawArgs: []string{"--interval=5", "my-pod", "curl", "-v", "https://ifconfig.me/"},
			wantOpts: whileOptions{
				podName:     "my-pod",
				interval:    "5",
				debugImage:  "nicolaka/netshoot:latest",
				commandArgs: []string{"curl", "-v", "https://ifconfig.me/"},
			},
		},
		{
			name:    "short -i before pod name and curl -i passthrough",
			rawArgs: []string{"-i", "5", "my-pod", "curl", "-i", "https://ifconfig.me/"},
			wantOpts: whileOptions{
				podName:     "my-pod",
				interval:    "5",
				debugImage:  "nicolaka/netshoot:latest",
				commandArgs: []string{"curl", "-i", "https://ifconfig.me/"},
			},
		},
		{
			name:    "short -i=5 syntax before pod name",
			rawArgs: []string{"-i=5", "my-pod", "curl", "-v", "https://ifconfig.me/"},
			wantOpts: whileOptions{
				podName:     "my-pod",
				interval:    "5",
				debugImage:  "nicolaka/netshoot:latest",
				commandArgs: []string{"curl", "-v", "https://ifconfig.me/"},
			},
		},
		{
			name:    "fractional interval before pod name",
			rawArgs: []string{"--interval", "0.5", "my-pod", "curl", "-s", "https://ifconfig.me/"},
			wantOpts: whileOptions{
				podName:     "my-pod",
				interval:    "0.5",
				debugImage:  "nicolaka/netshoot:latest",
				commandArgs: []string{"curl", "-s", "https://ifconfig.me/"},
			},
		},
		{
			name:    "global flags and interval before pod name, command flags passed through verbatim",
			rawArgs: []string{"-n", "prod", "--interval", "3", "-c", "app", "my-pod", "curl", "-n", "-c", "/tmp/cookies", "-v", "-h", "--interval", "99", "https://ifconfig.me/"},
			wantOpts: whileOptions{
				podName:       "my-pod",
				interval:      "3",
				namespace:     "prod",
				containerName: "app",
				debugImage:    "nicolaka/netshoot:latest",
				commandArgs:   []string{"curl", "-n", "-c", "/tmp/cookies", "-v", "-h", "--interval", "99", "https://ifconfig.me/"},
			},
		},
		{
			name:    "double dash separator after pod name passes everything after",
			rawArgs: []string{"--interval", "2", "my-pod", "--", "curl", "-H", "Content-Type: application/json", "http://svc:8080"},
			wantOpts: whileOptions{
				podName:     "my-pod",
				interval:    "2",
				debugImage:  "nicolaka/netshoot:latest",
				commandArgs: []string{"curl", "-H", "Content-Type: application/json", "http://svc:8080"},
			},
		},
		{
			name:        "interval after pod name is rejected",
			rawArgs:     []string{"my-pod", "--interval", "5", "curl", "-v", "https://ifconfig.me/"},
			expectError: true,
			errContains: "must be placed before POD_NAME",
		},
		{
			name:        "namespace flag after pod name is rejected",
			rawArgs:     []string{"my-pod", "-n", "prod", "curl", "https://ifconfig.me/"},
			expectError: true,
			errContains: "must be placed before POD_NAME",
		},
		{
			name:        "missing command after pod name",
			rawArgs:     []string{"my-pod"},
			expectError: true,
			errContains: "command to execute is required",
		},
		{
			name:        "missing interval value before pod name",
			rawArgs:     []string{"--interval"},
			expectError: true,
			errContains: "requires a value",
		},
		{
			name:        "zero interval is rejected",
			rawArgs:     []string{"--interval", "0", "my-pod", "date"},
			expectError: true,
			errContains: "positive number",
		},
		{
			name:        "negative interval is rejected",
			rawArgs:     []string{"--interval", "-2", "my-pod", "date"},
			expectError: true,
			errContains: "positive number",
		},
		{
			name:        "non-numeric interval is rejected",
			rawArgs:     []string{"--interval", "5; echo pwned", "my-pod", "date"},
			expectError: true,
			errContains: "positive number",
		},
		{
			name:        "unknown flag before pod name is rejected",
			rawArgs:     []string{"--intervl", "5", "my-pod", "date"},
			expectError: true,
			errContains: "unknown flag",
		},
		{
			name:        "missing pod name when only global flag given",
			rawArgs:     []string{"-n", "prod"},
			expectError: true,
			errContains: "target pod name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, showHelp, err := parseWhileArgs(tt.rawArgs, baseDefaults)
			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil (opts=%+v)", tt.errContains, got)
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
			if !tt.wantHelp && !reflect.DeepEqual(got, tt.wantOpts) {
				t.Fatalf("got opts = %+v, want %+v", got, tt.wantOpts)
			}
		})
	}
}

func TestBuildWhileCommand(t *testing.T) {
	t.Run("multi-arg command preserves argument boundaries", func(t *testing.T) {
		got := buildWhileCommand("5", []string{"curl", "-v", "-H", "X-Key: a b", "https://ifconfig.me/"})
		want := []string{
			"sh", "-c",
			`trap 'exit 130' INT TERM; while true; do "$@"; echo; sleep 5; done`,
			"_",
			"curl", "-v", "-H", "X-Key: a b", "https://ifconfig.me/",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	t.Run("single shell string with operators runs directly in sh loop", func(t *testing.T) {
		got := buildWhileCommand("2", []string{"curl -s https://ifconfig.me/ && echo"})
		want := []string{
			"sh", "-c",
			`trap 'exit 130' INT TERM; while true; do curl -s https://ifconfig.me/ && echo; echo; sleep 2; done`,
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	t.Run("single simple binary uses positional params", func(t *testing.T) {
		got := buildWhileCommand("1", []string{"date"})
		want := []string{
			"sh", "-c",
			`trap 'exit 130' INT TERM; while true; do "$@"; echo; sleep 1; done`,
			"_",
			"date",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})
}

func TestBuildWhileCommandShellExecution(t *testing.T) {
	// Verify that positional parameter invocation in sh preserves spaces and special chars
	// and appends a trailing newline via echo.
	cmdSlice := buildWhileCommand("1", []string{"sh", "-c", `printf '[%s]' "$@"; exit 0`, "_", "arg 1", "arg'2"})
	// Replace `while true; do` with a single-iteration run so the test terminates deterministically.
	cmdSlice[2] = strings.Replace(cmdSlice[2], "while true; do", "for _i in 1; do", 1)
	cmdSlice[2] = strings.Replace(cmdSlice[2], "sleep 1;", "", 1)

	out, err := exec.Command(cmdSlice[0], cmdSlice[1:]...).CombinedOutput()
	if err != nil {
		t.Fatalf("shell execution failed: %v (output: %s)", err, string(out))
	}
	want := "[arg 1][arg'2]\n"
	if string(out) != want {
		t.Fatalf("output = %q, want %q", string(out), want)
	}
}

func TestWhileCmdValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "missing args shows help without error",
			args:        []string{"while"},
			expectError: false,
		},
		{
			name:        "missing pod name after interval returns error",
			args:        []string{"while", "--interval", "5"},
			expectError: true,
		},
		{
			name:        "interval placed after pod name returns error",
			args:        []string{"while", "my-pod", "--interval", "5", "curl", "https://ifconfig.me/"},
			expectError: true,
		},
		{
			name:        "invalid interval before pod name returns error",
			args:        []string{"while", "--interval", "abc", "my-pod", "curl", "https://ifconfig.me/"},
			expectError: true,
		},
		{
			name:        "zero interval before pod name returns error",
			args:        []string{"while", "-i", "0", "my-pod", "curl", "https://ifconfig.me/"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			RootCmd.SetOut(buf)
			RootCmd.SetErr(buf)
			RootCmd.SetArgs(tt.args)

			err := RootCmd.Execute()
			if tt.expectError && err == nil {
				t.Errorf("expected error for args %v, but got nil", tt.args)
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error for args %v: %v", tt.args, err)
			}
		})
	}
}
