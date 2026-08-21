package capture

import (
	"fmt"
	"strings"
)

// Mode defines the packet capture execution strategy
type Mode string

const (
	ModeAuto      Mode = "auto"
	ModeEphemeral Mode = "ephemeral"
	ModeDirect    Mode = "direct"
)

// Options holds all parameters required for packet capturing
type Options struct {
	// Kubernetes target options
	PodName       string
	ContainerName string
	Namespace     string
	Kubeconfig    string
	KubeContext   string

	// Capture options
	Interface  string
	Filter     string
	Mode       Mode
	DebugImage string

	// Output options
	OutputFile    string
	WiresharkPath string

	// General
	Verbose bool
}

// NewDefaultOptions returns options with reasonable defaults
func NewDefaultOptions() *Options {
	return &Options{
		Interface:  "any",
		Mode:       ModeAuto,
		DebugImage: "nicolaka/netshoot:latest",
		Verbose:    false,
	}
}

// Validate checks whether essential parameters are specified
func (o *Options) Validate() error {
	if strings.TrimSpace(o.PodName) == "" {
		return fmt.Errorf("pod name is required")
	}

	switch o.Mode {
	case ModeAuto, ModeEphemeral, ModeDirect:
		// Valid
	default:
		return fmt.Errorf("invalid mode '%s': must be one of 'auto', 'ephemeral', 'direct'", o.Mode)
	}

	return nil
}
