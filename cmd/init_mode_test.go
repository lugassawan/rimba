package cmd

import (
	"strings"
	"testing"
)

func TestInitModeValidate(t *testing.T) {
	tests := []struct {
		name    string
		mode    initMode
		wantErr string // empty = expect nil
	}{
		{name: "empty mode", mode: initMode{}},
		{name: "global only", mode: initMode{global: true}},
		{name: "global + uninstall", mode: initMode{global: true, uninstall: true}},
		{name: "agents only", mode: initMode{agents: true}},
		{name: "agents + local", mode: initMode{agents: true, local: true}},
		{name: "agents + uninstall", mode: initMode{agents: true, uninstall: true}},
		{name: "agents + local + uninstall", mode: initMode{agents: true, local: true, uninstall: true}},
		{
			name:    "global + local without agents (validate fires before global branch)",
			mode:    initMode{global: true, local: true},
			wantErr: "--local requires --agents",
		},
		{
			name:    "local without agents",
			mode:    initMode{local: true},
			wantErr: "--local requires --agents",
		},
		{
			name:    "uninstall without target",
			mode:    initMode{uninstall: true},
			wantErr: "--uninstall requires",
		},
		{
			name:    "local + uninstall without agents (local check fires first)",
			mode:    initMode{local: true, uninstall: true},
			wantErr: "--local requires --agents",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mode.validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("validate() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validate() = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("validate() = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}
