package output

import (
	"fmt"
	"io"
	"os"
)

// OutputHandler manages the destination of the pcap data stream
type OutputHandler struct {
	Writer   io.Writer
	Closer   io.Closer
	ExitChan <-chan struct{}
	IsPipe   bool
	Summary  string
}

// SetupOutput initializes the appropriate output sink based on user options
func SetupOutput(outputFile string, customWiresharkPath string) (*OutputHandler, error) {
	// Case 1: Stream to stdout (for piping e.g. kubectl net cap ... -o - | tshark -r -)
	if outputFile == "-" {
		return &OutputHandler{
			Writer:   os.Stdout,
			Closer:   nil,
			ExitChan: nil,
			IsPipe:   true,
			Summary:  "Streaming pcap directly to stdout",
		}, nil
	}

	// Case 2: Save to a local .pcap file
	if outputFile != "" {
		file, err := os.Create(outputFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create output file '%s': %w", outputFile, err)
		}
		return &OutputHandler{
			Writer:   file,
			Closer:   file,
			ExitChan: nil,
			IsPipe:   false,
			Summary:  fmt.Sprintf("Writing packets to file: %s", outputFile),
		}, nil
	}

	// Case 3: Default - Launch Wireshark GUI
	wsPath, err := FindWireshark(customWiresharkPath)
	if err != nil {
		return nil, err
	}

	wsProcess, err := StartWireshark(wsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to launch Wireshark: %w", err)
	}

	return &OutputHandler{
		Writer:   wsProcess,
		Closer:   wsProcess,
		ExitChan: wsProcess.ExitChan(),
		IsPipe:   false,
		Summary:  fmt.Sprintf("Piping packets to Wireshark: %s", wsPath),
	}, nil
}

// Close closes the underlying resource if applicable
func (h *OutputHandler) Close() error {
	if h.Closer != nil {
		return h.Closer.Close()
	}
	return nil
}
