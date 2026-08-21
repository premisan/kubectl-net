package capture

import (
	"testing"
)

func TestOptionsValidation(t *testing.T) {
	tests := []struct {
		name    string
		opts    *Options
		wantErr bool
	}{
		{
			name: "Valid default with pod",
			opts: &Options{
				PodName: "my-pod",
				Mode:    ModeAuto,
			},
			wantErr: false,
		},
		{
			name: "Missing pod name",
			opts: &Options{
				PodName: "",
				Mode:    ModeAuto,
			},
			wantErr: true,
		},
		{
			name: "Invalid mode",
			opts: &Options{
				PodName: "my-pod",
				Mode:    Mode("invalid-mode"),
			},
			wantErr: true,
		},
		{
			name: "Valid ephemeral mode",
			opts: &Options{
				PodName: "my-pod",
				Mode:    ModeEphemeral,
			},
			wantErr: false,
		},
		{
			name: "Valid direct mode",
			opts: &Options{
				PodName: "my-pod",
				Mode:    ModeDirect,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
