package operations_test

import (
	"errors"
	"testing"

	"github.com/lugassawan/rimba/internal/operations"
)

func ptr[T any](v T) *T { return &v }

func TestBuildDiskFootprint(t *testing.T) {
	sizes := []*int64{ptr(int64(1000)), ptr(int64(500)), nil}
	fp := operations.BuildDiskFootprint(sizes, 10_000, nil)

	if fp.MainBytes != 10_000 || fp.WorktreesBytes != 1500 || fp.TotalBytes != 11_500 {
		t.Errorf("fp = %+v, want main=10000 wt=1500 total=11500", fp)
	}
	if fp.MainErr != nil {
		t.Errorf("MainErr = %v, want nil", fp.MainErr)
	}
}

func TestBuildDiskFootprintMainError(t *testing.T) {
	want := errors.New("permission denied")
	fp := operations.BuildDiskFootprint([]*int64{ptr(int64(200))}, 0, want)

	if fp.MainBytes != 0 {
		t.Errorf("MainBytes = %d, want 0 when MainErr != nil", fp.MainBytes)
	}
	if fp.WorktreesBytes != 200 || fp.TotalBytes != 200 {
		t.Errorf("wt=%d total=%d, want 200/200 (main excluded)", fp.WorktreesBytes, fp.TotalBytes)
	}
	if !errors.Is(fp.MainErr, want) {
		t.Errorf("MainErr = %v, want %v", fp.MainErr, want)
	}
}

func TestBuildDiskFootprintAllNil(t *testing.T) {
	fp := operations.BuildDiskFootprint([]*int64{nil, nil}, 0, nil)
	if fp.TotalBytes != 0 || fp.WorktreesBytes != 0 {
		t.Errorf("fp = %+v, want zeros", fp)
	}
}
