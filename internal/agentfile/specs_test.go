package agentfile

import "testing"

func TestSpecsReturnsSeven(t *testing.T) {
	if got := len(ProjectSpecs()); got != 7 {
		t.Fatalf("Specs() returned %d items, want 7", got)
	}
}

func TestSpecsContentNotEmpty(t *testing.T) {
	for _, spec := range ProjectSpecs() {
		content := spec.Content()
		if content == "" {
			t.Errorf("Spec %q returned empty content", spec.RelPath)
		}
	}
}

func TestGlobalSpecsCountIsSeven(t *testing.T) {
	if got := len(GlobalSpecs()); got != 7 {
		t.Fatalf("GlobalSpecs() returned %d items, want 7", got)
	}
}

func TestProjectSpecsCountIsSeven(t *testing.T) {
	if got := len(ProjectSpecs()); got != 7 {
		t.Fatalf("ProjectSpecs() returned %d items, want 7", got)
	}
}

func TestGlobalSpecsContentNotEmpty(t *testing.T) {
	for _, spec := range GlobalSpecs() {
		if spec.Content() == "" {
			t.Errorf("GlobalSpec %q returned empty content", spec.RelPath)
		}
	}
}

func TestProjectSpecsContentNotEmpty(t *testing.T) {
	for _, spec := range ProjectSpecs() {
		if spec.Content() == "" {
			t.Errorf("ProjectSpec %q returned empty content", spec.RelPath)
		}
	}
}
