package e2e_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lugassawan/rimba/testutil"
)

func TestFleetSpawnAndStatus(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	// Create a mock agent script that echoes and exits.
	mockAgent := filepath.Join(t.TempDir(), "mock-agent")
	if err := os.WriteFile(mockAgent, []byte("#!/bin/sh\necho 'agent running'\nsleep 1\n"), 0755); err != nil {
		t.Fatalf("write mock agent: %v", err)
	}

	// Configure fleet to use the mock agent.
	cfg := filepath.Join(repo, configFile)
	content, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	content = append(content, []byte("\n[fleet]\ndefault_agent = \"generic\"\n\n[fleet.agents.generic]\ncommand = \""+mockAgent+"\"\n")...)
	if err := os.WriteFile(cfg, content, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Spawn with generic agent
	r := rimbaSuccess(t, repo, "fleet", "spawn", "--agent", "generic", taskFleet)
	assertContains(t, r.Stdout, taskFleet)

	// Give agent a moment to start
	time.Sleep(100 * time.Millisecond)

	// Check status
	r = rimbaSuccess(t, repo, "fleet", "status")
	assertContains(t, r.Stdout, taskFleet)
}

func TestFleetSpawnFromManifest(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	// Create manifest
	manifest := filepath.Join(repo, fileFleetToml)
	testutil.CreateFile(t, repo, fileFleetToml,
		"[[tasks]]\nname = \"manifest-task\"\nagent = \"generic\"\n")

	_ = manifest // used implicitly via file creation

	r := rimbaSuccess(t, repo, "fleet", "spawn", "--manifest", fileFleetToml)
	assertContains(t, r.Stdout, "manifest-task")
}

func TestFleetNoArgs(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	r := rimbaFail(t, repo, "fleet", "spawn")
	assertContains(t, r.Stderr, "provide task names or --manifest")
}

func TestFleetStatusEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	r := rimbaSuccess(t, repo, "fleet", "status")
	assertContains(t, r.Stdout, "No fleet tasks found")
}
