package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// Envelope wraps all JSON command output with metadata.
type Envelope struct {
	Version string `json:"version"`
	Command string `json:"command"`
	Data    any    `json:"data"`
}

// ErrorEnvelope wraps JSON error output.
type ErrorEnvelope struct {
	Version string `json:"version"`
	Command string `json:"command"`
	Error   string `json:"error"`
	Code    string `json:"code"`
}

// Error code constants.
const (
	ErrGeneral = "GENERAL_ERROR"
)

// SilentError signals a non-zero exit without additional error output.
// Used when the command has already written its JSON output.
type SilentError struct{ ExitCode int }

func (e *SilentError) Error() string { return fmt.Sprintf("exit %d", e.ExitCode) }

// WriteJSON writes a JSON envelope to w with pretty-printed indentation.
func WriteJSON(w io.Writer, version, command string, data any) error {
	env := Envelope{
		Version: version,
		Command: command,
		Data:    data,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(env)
}

// WriteJSONError writes a JSON error envelope to w.
func WriteJSONError(w io.Writer, version, command, errMsg, code string) error {
	env := ErrorEnvelope{
		Version: version,
		Command: command,
		Error:   errMsg,
		Code:    code,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(env)
}

// IsJSON returns true if the --json flag is set on the given command.
// Returns false if the flag is not registered.
func IsJSON(cmd *cobra.Command) bool {
	f := cmd.Flags().Lookup("json")
	if f == nil {
		// Try persistent flags (inherited from parent)
		f = cmd.InheritedFlags().Lookup("json")
	}
	if f == nil {
		return false
	}
	return f.Value.String() == "true"
}
