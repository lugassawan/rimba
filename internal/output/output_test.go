package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
)

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]string{"key": "value"}

	err := WriteJSON(&buf, "1.0.0", "list", data)
	if err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var env Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if env.Version != "1.0.0" {
		t.Errorf("version = %q, want %q", env.Version, "1.0.0")
	}
	if env.Command != "list" {
		t.Errorf("command = %q, want %q", env.Command, "list")
	}

	m, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", env.Data)
	}
	if m["key"] != "value" {
		t.Errorf("data[key] = %v, want %q", m["key"], "value")
	}
}

func TestWriteJSONError(t *testing.T) {
	var buf bytes.Buffer

	err := WriteJSONError(&buf, "1.0.0", "list", "something broke", ErrGeneral)
	if err != nil {
		t.Fatalf("WriteJSONError: %v", err)
	}

	var env ErrorEnvelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if env.Version != "1.0.0" {
		t.Errorf("version = %q, want %q", env.Version, "1.0.0")
	}
	if env.Command != "list" {
		t.Errorf("command = %q, want %q", env.Command, "list")
	}
	if env.Error != "something broke" {
		t.Errorf("error = %q, want %q", env.Error, "something broke")
	}
	if env.Code != ErrGeneral {
		t.Errorf("code = %q, want %q", env.Code, ErrGeneral)
	}
}

func TestIsJSON_FlagSet(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool("json", false, "")
	_ = cmd.Flags().Set("json", "true")

	if !IsJSON(cmd) {
		t.Error("IsJSON should return true when --json is set")
	}
}

func TestIsJSON_FlagNotSet(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool("json", false, "")

	if IsJSON(cmd) {
		t.Error("IsJSON should return false when --json is not set")
	}
}

func TestIsJSON_NoFlag(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}

	if IsJSON(cmd) {
		t.Error("IsJSON should return false when flag is not registered")
	}
}

func TestIsJSON_InheritedPersistentFlag(t *testing.T) {
	parent := &cobra.Command{Use: "root"}
	parent.PersistentFlags().Bool("json", false, "")
	_ = parent.PersistentFlags().Set("json", "true")

	child := &cobra.Command{Use: "child"}
	parent.AddCommand(child)

	// Need to parse flags for inheritance to work
	_ = parent.ParseFlags([]string{})

	if !IsJSON(child) {
		t.Error("IsJSON should return true for inherited persistent flag")
	}
}

func TestSilentError(t *testing.T) {
	err := &SilentError{ExitCode: 1}
	if err.Error() != "exit 1" {
		t.Errorf("Error() = %q, want %q", err.Error(), "exit 1")
	}
}

func TestWriteJSON_PrettyPrinted(t *testing.T) {
	var buf bytes.Buffer
	err := WriteJSON(&buf, "1.0.0", "test", []string{"a"})
	if err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	// Should contain indentation
	out := buf.String()
	if len(out) == 0 {
		t.Fatal("output is empty")
	}
	if out[0] != '{' {
		t.Errorf("output should start with '{', got %q", out[:1])
	}
	// Check for indentation marker
	if !bytes.Contains(buf.Bytes(), []byte("  ")) {
		t.Error("output should be pretty-printed with indentation")
	}
}
