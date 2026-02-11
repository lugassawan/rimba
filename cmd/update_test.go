package cmd

import (
	"strings"
	"testing"
)

func TestUpdateDevVersionGuard(t *testing.T) {
	orig := version
	version = "dev"
	defer func() { version = orig }()

	cmd, buf := newTestCmd()
	cmd.Flags().Bool("force", false, "")

	err := updateCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("updateCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "development build") {
		t.Errorf("output = %q, want 'development build'", out)
	}
	if !strings.Contains(out, "--force") {
		t.Errorf("output = %q, want '--force'", out)
	}
}

func TestUpdateDevVersionEmpty(t *testing.T) {
	orig := version
	version = ""
	defer func() { version = orig }()

	cmd, buf := newTestCmd()
	cmd.Flags().Bool("force", false, "")

	err := updateCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("updateCmd.RunE: %v", err)
	}
	if !strings.Contains(buf.String(), "development build") {
		t.Errorf("output = %q, want 'development build'", buf.String())
	}
}
