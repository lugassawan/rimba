package agentfile

import "testing"

func TestSpecsContentNotEmpty(t *testing.T) {
	for _, spec := range ProjectSpecs() {
		content := spec.Content()
		if content == "" {
			t.Errorf("Spec %q returned empty content", spec.RelPath)
		}
	}
}

func TestGlobalSpecsCount(t *testing.T) {
	if got := len(GlobalSpecs()); got != 9 {
		t.Fatalf("GlobalSpecs() returned %d items, want 9", got)
	}
}

func TestProjectSpecsCount(t *testing.T) {
	if got := len(ProjectSpecs()); got != 8 {
		t.Fatalf("ProjectSpecs() returned %d items, want 8", got)
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
