package config_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

func TestNormalizeHookStagesNil(t *testing.T) {
	stages, err := config.NormalizeHookStages(nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stages != nil {
		t.Errorf("stages = %v, want nil", stages)
	}
}

func TestNormalizeHookStagesNativeGoStringSlice(t *testing.T) {
	// A native []string (set directly in code, e.g. by tests or
	// DefaultConfig) must normalize the same way as its []any-decoded
	// TOML equivalent — this is the shape config.Config{PostCreate:
	// []string{...}} literals produce, not just TOML.Unmarshal output.
	raw := []string{"cmd1", "cmd2"}

	serial, err := config.NormalizeHookStages(raw, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := [][]string{{"cmd1"}, {"cmd2"}}; !reflect.DeepEqual(serial, want) {
		t.Errorf("serial stages = %v, want %v", serial, want)
	}

	parallel, err := config.NormalizeHookStages(raw, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := [][]string{{"cmd1", "cmd2"}}; !reflect.DeepEqual(parallel, want) {
		t.Errorf("parallel stages = %v, want %v", parallel, want)
	}
}

func TestNormalizeHookStagesNativeGoEmptyStringSlice(t *testing.T) {
	stages, err := config.NormalizeHookStages([]string{}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stages != nil {
		t.Errorf("stages = %v, want nil", stages)
	}
}

func TestNormalizeHookStagesNativeGoNestedSlice(t *testing.T) {
	// A native [][]string (already-canonical stages, e.g. constructed
	// directly by a caller) must be returned as given.
	raw := [][]string{{"cmd1", "cmd2"}, {"cmd3"}}

	stages, err := config.NormalizeHookStages(raw, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(stages, raw) {
		t.Errorf("stages = %v, want %v", stages, raw)
	}
}

func TestNormalizeHookStagesNativeGoEmptyNestedSlice(t *testing.T) {
	stages, err := config.NormalizeHookStages([][]string{}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stages != nil {
		t.Errorf("stages = %v, want nil", stages)
	}
}

func TestConfigPostCreateStagesFromNativeGoStringSlice(t *testing.T) {
	// Exercises the DefaultConfig()/test-fixture path: PostCreate assigned
	// as a Go-level []string literal, not loaded from a TOML file.
	cfg := &config.Config{PostCreate: []string{"echo hello"}}
	stages, err := cfg.PostCreateStages()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := [][]string{{"echo hello"}}
	if !reflect.DeepEqual(stages, want) {
		t.Errorf("stages = %v, want %v", stages, want)
	}
}

func TestConfigPostCreateStagesWrapsError(t *testing.T) {
	cfg := &config.Config{PostCreate: "not-an-array"}
	_, err := cfg.PostCreateStages()
	if err == nil {
		t.Fatal("expected error for malformed post_create")
	}
	if !strings.Contains(err.Error(), "post_create") {
		t.Errorf("error = %q, want it prefixed with 'post_create'", err.Error())
	}
}

func TestConfigPostRenameStagesWrapsError(t *testing.T) {
	cfg := &config.Config{PostRename: "not-an-array"}
	_, err := cfg.PostRenameStages()
	if err == nil {
		t.Fatal("expected error for malformed post_rename")
	}
	if !strings.Contains(err.Error(), "post_rename") {
		t.Errorf("error = %q, want it prefixed with 'post_rename'", err.Error())
	}
}

func TestNormalizeHookStagesFlatSerial(t *testing.T) {
	raw := []any{"cmd1", "cmd2"}
	stages, err := config.NormalizeHookStages(raw, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := [][]string{{"cmd1"}, {"cmd2"}}
	if !reflect.DeepEqual(stages, want) {
		t.Errorf("stages = %v, want %v", stages, want)
	}
}

func TestNormalizeHookStagesFlatParallel(t *testing.T) {
	raw := []any{"cmd1", "cmd2"}
	stages, err := config.NormalizeHookStages(raw, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := [][]string{{"cmd1", "cmd2"}}
	if !reflect.DeepEqual(stages, want) {
		t.Errorf("stages = %v, want %v", stages, want)
	}
}

func TestNormalizeHookStagesNested(t *testing.T) {
	raw := []any{
		[]any{"cmd1", "cmd2"},
		[]any{"cmd3"},
	}
	// flatParallel is irrelevant once the shape is nested.
	stages, err := config.NormalizeHookStages(raw, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := [][]string{{"cmd1", "cmd2"}, {"cmd3"}}
	if !reflect.DeepEqual(stages, want) {
		t.Errorf("stages = %v, want %v", stages, want)
	}
}

func TestNormalizeHookStagesNestedIgnoresFlatParallel(t *testing.T) {
	raw := []any{
		[]any{"cmd1", "cmd2"},
		[]any{"cmd3"},
	}
	stagesA, err := config.NormalizeHookStages(raw, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stagesB, err := config.NormalizeHookStages(raw, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(stagesA, stagesB) {
		t.Errorf("nested result differs by flatParallel: %v vs %v", stagesA, stagesB)
	}
}

func TestNormalizeHookStagesEmptyGroup(t *testing.T) {
	raw := []any{
		[]any{},
		[]any{"cmd1"},
	}
	stages, err := config.NormalizeHookStages(raw, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := [][]string{{}, {"cmd1"}}
	if !reflect.DeepEqual(stages, want) {
		t.Errorf("stages = %v, want %v", stages, want)
	}
}

func TestNormalizeHookStagesEmptyTopLevel(t *testing.T) {
	stages, err := config.NormalizeHookStages([]any{}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stages != nil {
		t.Errorf("stages = %v, want nil", stages)
	}
}

func TestNormalizeHookStagesMixedFlatAndNestedRejected(t *testing.T) {
	raw := []any{"cmd1", []any{"cmd2"}}
	_, err := config.NormalizeHookStages(raw, false)
	if err == nil {
		t.Fatal("expected error for mixed flat/nested entries")
	}
	if !strings.Contains(err.Error(), "mixed") {
		t.Errorf("error = %q, want it to mention 'mixed'", err.Error())
	}
}

func TestNormalizeHookStagesDeeplyNestedRejected(t *testing.T) {
	raw := []any{
		[]any{[]any{"cmd1"}},
	}
	_, err := config.NormalizeHookStages(raw, false)
	if err == nil {
		t.Fatal("expected error for depth > 2 nesting")
	}
}

func TestNormalizeHookStagesNonStringElementRejected(t *testing.T) {
	raw := []any{42}
	_, err := config.NormalizeHookStages(raw, false)
	if err == nil {
		t.Fatal("expected error for non-string element")
	}
}

func TestNormalizeHookStagesWrongTopLevelTypeRejected(t *testing.T) {
	_, err := config.NormalizeHookStages("not-an-array", false)
	if err == nil {
		t.Fatal("expected error for non-array top-level value")
	}
}

func TestConfigPostCreateStagesUsesHooksParallel(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }
	cfg := &config.Config{
		PostCreate: []any{"cmd1", "cmd2"},
		Hooks:      &config.HooksConfig{Parallel: boolPtr(true)},
	}
	stages, err := cfg.PostCreateStages()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := [][]string{{"cmd1", "cmd2"}}
	if !reflect.DeepEqual(stages, want) {
		t.Errorf("stages = %v, want %v", stages, want)
	}
}

func TestConfigPostRenameStagesIgnoresHooksParallel(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }
	cfg := &config.Config{
		PostRename: []any{"cmd1", "cmd2"},
		Hooks:      &config.HooksConfig{Parallel: boolPtr(true)},
	}
	stages, err := cfg.PostRenameStages()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// post_rename's flat form stays fully serial regardless of [hooks] parallel —
	// only the nested/staged shape opts a post_rename config into parallelism.
	want := [][]string{{"cmd1"}, {"cmd2"}}
	if !reflect.DeepEqual(stages, want) {
		t.Errorf("stages = %v, want %v", stages, want)
	}
}

func TestPostCreateStagesFromRealTOMLFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, config.FileName)
	toml := "post_create = [\n  [\"cmd1\", \"cmd2\"],\n  [\"cmd3\"],\n]\n" +
		"post_rename = [\"cmd4\", \"cmd5\"]\n"
	if err := os.WriteFile(path, []byte(toml), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	postCreate, err := cfg.PostCreateStages()
	if err != nil {
		t.Fatalf("PostCreateStages: %v", err)
	}
	wantCreate := [][]string{{"cmd1", "cmd2"}, {"cmd3"}}
	if !reflect.DeepEqual(postCreate, wantCreate) {
		t.Errorf("PostCreateStages() = %v, want %v", postCreate, wantCreate)
	}

	postRename, err := cfg.PostRenameStages()
	if err != nil {
		t.Fatalf("PostRenameStages: %v", err)
	}
	wantRename := [][]string{{"cmd4"}, {"cmd5"}}
	if !reflect.DeepEqual(postRename, wantRename) {
		t.Errorf("PostRenameStages() = %v, want %v", postRename, wantRename)
	}
}

func TestConfigPostRenameStagesNestedStillParallel(t *testing.T) {
	cfg := &config.Config{
		PostRename: []any{[]any{"cmd1", "cmd2"}},
	}
	stages, err := cfg.PostRenameStages()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := [][]string{{"cmd1", "cmd2"}}
	if !reflect.DeepEqual(stages, want) {
		t.Errorf("stages = %v, want %v", stages, want)
	}
}
