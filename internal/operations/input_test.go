package operations_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lugassawan/rimba/internal/operations"
)

func TestResolveTaskInput(t *testing.T) {
	// Create a temp dir with a service subdirectory
	repoRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoRoot, "auth-api"), 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		input       string
		wantService string
		wantTask    string
	}{
		{
			name:        "simple task",
			input:       "my-task",
			wantService: "",
			wantTask:    "my-task",
		},
		{
			name:        "known prefix",
			input:       "feature/my-task",
			wantService: "",
			wantTask:    "my-task",
		},
		{
			name:        "known prefix with multi-segment task",
			input:       "feature/auth-redirect/part-1",
			wantService: "",
			wantTask:    "auth-redirect-part-1",
		},
		{
			name:        "bugfix prefix",
			input:       "bugfix/crash-fix",
			wantService: "",
			wantTask:    "crash-fix",
		},
		{
			name:        "valid service directory",
			input:       "auth-api/my-task",
			wantService: "auth-api",
			wantTask:    "my-task",
		},
		{
			name:        "valid service with multi-segment task",
			input:       "auth-api/auth-redirect/part-1",
			wantService: "auth-api",
			wantTask:    "auth-redirect-part-1",
		},
		{
			name:        "non-existent directory falls back to standard",
			input:       "no-such-dir/my-task",
			wantService: "",
			wantTask:    "no-such-dir-my-task",
		},
		{
			name:        "non-existent dir with multi-segment",
			input:       "no-such-dir/part-1/part-2",
			wantService: "",
			wantTask:    "no-such-dir-part-1-part-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, task := operations.ResolveTaskInput(tt.input, repoRoot)
			if service != tt.wantService || task != tt.wantTask {
				t.Errorf("ResolveTaskInput(%q) = (%q, %q), want (%q, %q)",
					tt.input, service, task, tt.wantService, tt.wantTask)
			}
		})
	}
}
