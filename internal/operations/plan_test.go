package operations

import (
	"errors"
	"testing"
)

func TestPlanDoDryRun(t *testing.T) {
	tests := []struct {
		name       string
		dryRun     bool
		wantCalled bool
	}{
		{"skips action when DryRun=true", true, false},
		{"executes action when DryRun=false", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Plan{DryRun: tt.dryRun}
			called := false
			err := p.Do("step", func() error {
				called = true
				return nil
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if called != tt.wantCalled {
				t.Errorf("called=%v, want %v", called, tt.wantCalled)
			}
			if len(p.Steps) != 1 || p.Steps[0] != "step" {
				t.Errorf("steps=%v, want [step]", p.Steps)
			}
		})
	}
}

func TestPlanDoPropagatesError(t *testing.T) {
	p := &Plan{DryRun: false}
	wantErr := errors.New("boom")
	err := p.Do("failing step", func() error { return wantErr })
	if !errors.Is(err, wantErr) {
		t.Fatalf("error=%v, want %v", err, wantErr)
	}
	if len(p.Steps) != 1 {
		t.Fatalf("expected step recorded even on error, got %v", p.Steps)
	}
}

func TestPlanDoAccumulatesSteps(t *testing.T) {
	p := &Plan{DryRun: true}
	for _, desc := range []string{"a", "b", "c"} {
		d := desc
		if err := p.Do(d, func() error { return nil }); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if len(p.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d: %v", len(p.Steps), p.Steps)
	}
}
