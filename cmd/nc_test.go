package cmd

import (
	"bytes"
	"testing"
)

func TestNcCmdValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "missing args without flag",
			args:        []string{"nc"},
			expectError: false, // shows help
		},
		{
			name:        "only pod name in connect mode",
			args:        []string{"nc", "my-pod"},
			expectError: true,
		},
		{
			name:        "only pod name and host in connect mode",
			args:        []string{"nc", "my-pod", "10.0.0.1"},
			expectError: true,
		},
		{
			name:        "invalid port number in connect mode",
			args:        []string{"nc", "my-pod", "10.0.0.1", "99999"},
			expectError: true,
		},
		{
			name:        "listen mode without port",
			args:        []string{"nc", "my-pod", "-l"},
			expectError: true,
		},
		{
			name:        "listen mode with invalid port",
			args:        []string{"nc", "my-pod", "-l", "invalid-port"},
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
		})
	}
}
