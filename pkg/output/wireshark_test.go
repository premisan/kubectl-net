package output

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindWiresharkCustomPath(t *testing.T) {
	// Create a dummy temp binary
	tmpDir := t.TempDir()
	dummyBin := filepath.Join(tmpDir, "dummy_wireshark")
	if err := os.WriteFile(dummyBin, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("failed to create dummy binary: %v", err)
	}

	found, err := FindWireshark(dummyBin)
	if err != nil {
		t.Fatalf("expected to find custom binary, got error: %v", err)
	}
	if found != dummyBin {
		t.Errorf("expected '%s', got '%s'", dummyBin, found)
	}

	// Non-existent custom path
	_, err = FindWireshark(filepath.Join(tmpDir, "nonexistent"))
	if err == nil {
		t.Errorf("expected error for nonexistent custom path, got nil")
	}
}
