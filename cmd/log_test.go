package cmd

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lugassawan/rimba/internal/termcolor"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
		err   bool
	}{
		{"7d", 7 * 24 * time.Hour, false},
		{"2w", 14 * 24 * time.Hour, false},
		{"3h", 3 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"", 0, true},
		{"x", 0, true},
		{"7x", 0, true},
		{"abc", 0, true},
		{"-5d", 0, true},
		{"0d", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseDuration(tt.input)
			if (err != nil) != tt.err {
				t.Errorf("parseDuration(%q) error = %v, wantErr %v", tt.input, err, tt.err)
				return
			}
			if got != tt.want {
				t.Errorf("parseDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestLogNoWorktrees(t *testing.T) {
	restore := overrideNewRunner(&mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case args[0] == cmdRevParse && args[1] == cmdShowToplevel:
				return repoPath, nil
			case args[0] == cmdSymbolicRef:
				return refsRemotesOriginMain, nil
			case args[0] == cmdWorktreeTest && args[1] == cmdList:
				return wtRepo + headMainBlock, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	})
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Int(flagLimit, 0, "")
	cmd.Flags().String(flagSince, "", "")

	if err := logCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("logCmd.RunE: %v", err)
	}

	if !strings.Contains(buf.String(), "No worktrees found") {
		t.Errorf("expected 'No worktrees found', got: %q", buf.String())
	}
}

func TestLogWithWorktrees(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Add(-2*time.Hour).Unix(), 10)

	restore := overrideNewRunner(&mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case args[0] == cmdRevParse && args[1] == cmdShowToplevel:
				return repoPath, nil
			case args[0] == cmdSymbolicRef:
				return refsRemotesOriginMain, nil
			case args[0] == cmdWorktreeTest && args[1] == cmdList:
				return wtRepo + headMainBlock + "\n" +
					wtFeatureLogin + "\n" + headDEF456 + "\n" + branchRefFeatureLogin + "\n", nil
			case args[0] == cmdLog:
				return ts + "\tfix login bug", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	})
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Int(flagLimit, 0, "")
	cmd.Flags().String(flagSince, "", "")

	if err := logCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("logCmd.RunE: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Recent commits") {
		t.Errorf("expected 'Recent commits' header, got: %q", output)
	}
	if !strings.Contains(output, taskLogin) {
		t.Errorf("expected task 'login', got: %q", output)
	}
	if !strings.Contains(output, "fix login bug") {
		t.Errorf("expected commit subject, got: %q", output)
	}
}

func TestAgeColorValues(t *testing.T) {
	tests := []struct {
		name string
		age  time.Duration
		want termcolor.Color
	}{
		{"recent", 1 * time.Hour, termcolor.Green},
		{"few days", 5 * 24 * time.Hour, termcolor.Yellow},
		{"old", 30 * 24 * time.Hour, termcolor.Red},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ageColor(time.Now().Add(-tt.age))
			if got != tt.want {
				t.Errorf("ageColor(-%v) = %q, want %q", tt.age, got, tt.want)
			}
		})
	}
}
