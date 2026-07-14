package git

import (
	"context"
	"errors"
	"testing"
)

func TestClassifyPorcelainDeletionsMultipleDeletions(t *testing.T) {
	r := &mockRunner{
		runInDir: func(_ string, _ ...string) (string, error) {
			return " D file1.txt\n D file2.txt\n D file3.txt\n", nil
		},
	}

	status, err := ClassifyPorcelainDeletions(context.Background(), r, fakeDir)
	if err != nil {
		t.Fatalf("ClassifyPorcelainDeletions: %v", err)
	}
	if !status.AllDeletions() {
		t.Error("expected AllDeletions() to return true for only deletions")
	}
	if status.Deleted != 3 {
		t.Errorf("deleted = %d, want 3", status.Deleted)
	}
	if status.Other != 0 {
		t.Errorf("other = %d, want 0", status.Other)
	}
}

func TestClassifyPorcelainDeletionsMixedChanges(t *testing.T) {
	r := &mockRunner{
		runInDir: func(_ string, _ ...string) (string, error) {
			return " D deleted.txt\n M modified.txt\n", nil
		},
	}

	status, err := ClassifyPorcelainDeletions(context.Background(), r, fakeDir)
	if err != nil {
		t.Fatalf("ClassifyPorcelainDeletions: %v", err)
	}
	if status.AllDeletions() {
		t.Error("expected AllDeletions() to return false with mixed changes")
	}
	if status.Deleted != 1 {
		t.Errorf("deleted = %d, want 1", status.Deleted)
	}
	if status.Other != 1 {
		t.Errorf("other = %d, want 1", status.Other)
	}
}

func TestClassifyPorcelainDeletionsStagedDeletion(t *testing.T) {
	r := &mockRunner{
		runInDir: func(_ string, _ ...string) (string, error) {
			return "D  staged-deletion.txt\n", nil
		},
	}

	status, err := ClassifyPorcelainDeletions(context.Background(), r, fakeDir)
	if err != nil {
		t.Fatalf("ClassifyPorcelainDeletions: %v", err)
	}
	if status.AllDeletions() {
		t.Error("expected AllDeletions() to return false for staged deletion (D in column 0)")
	}
	if status.Deleted != 0 {
		t.Errorf("deleted = %d, want 0", status.Deleted)
	}
	if status.Other != 1 {
		t.Errorf("other = %d, want 1", status.Other)
	}
}

func TestClassifyPorcelainDeletionsUntrackedFiles(t *testing.T) {
	r := &mockRunner{
		runInDir: func(_ string, _ ...string) (string, error) {
			return "?? untracked.txt\n", nil
		},
	}

	status, err := ClassifyPorcelainDeletions(context.Background(), r, fakeDir)
	if err != nil {
		t.Fatalf("ClassifyPorcelainDeletions: %v", err)
	}
	if status.AllDeletions() {
		t.Error("expected AllDeletions() to return false for untracked files")
	}
	if status.Deleted != 0 {
		t.Errorf("deleted = %d, want 0", status.Deleted)
	}
	if status.Other != 1 {
		t.Errorf("other = %d, want 1", status.Other)
	}
}

func TestClassifyPorcelainDeletionsConflict(t *testing.T) {
	r := &mockRunner{
		runInDir: func(_ string, _ ...string) (string, error) {
			return "DU conflict.txt\n", nil
		},
	}

	status, err := ClassifyPorcelainDeletions(context.Background(), r, fakeDir)
	if err != nil {
		t.Fatalf("ClassifyPorcelainDeletions: %v", err)
	}
	if status.AllDeletions() {
		t.Error("expected AllDeletions() to return false for conflict")
	}
	if status.Deleted != 0 {
		t.Errorf("deleted = %d, want 0", status.Deleted)
	}
	if status.Other != 1 {
		t.Errorf("other = %d, want 1", status.Other)
	}
}

func TestClassifyPorcelainDeletionsEmpty(t *testing.T) {
	r := &mockRunner{
		runInDir: func(_ string, _ ...string) (string, error) {
			return "", nil
		},
	}

	status, err := ClassifyPorcelainDeletions(context.Background(), r, fakeDir)
	if err != nil {
		t.Fatalf("ClassifyPorcelainDeletions: %v", err)
	}
	if status.AllDeletions() {
		t.Error("expected AllDeletions() to return false for empty status")
	}
	if status.Deleted != 0 {
		t.Errorf("deleted = %d, want 0", status.Deleted)
	}
	if status.Other != 0 {
		t.Errorf("other = %d, want 0", status.Other)
	}
}

func TestClassifyPorcelainDeletionsRunInDirError(t *testing.T) {
	r := &mockRunner{
		runInDir: func(_ string, _ ...string) (string, error) {
			return "", errors.New(errNotARepo)
		},
	}

	_, err := ClassifyPorcelainDeletions(context.Background(), r, fakeDir)
	if err == nil {
		t.Fatal("expected error from RunInDir failure")
	}
	assertContains(t, err, errNotARepo)
}
