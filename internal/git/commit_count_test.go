package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/testutil"
)

func TestCommitCountSinceCountsRecent(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	// NewTestRepo already made 1 initial commit "now".
	// Add 2 more recent commits.
	for i := range 2 {
		testutil.CreateFile(t, repo, filepath.Join(".", "f"+string(rune('a'+i))), "x")
		testutil.GitCmd(t, repo, "add", ".")
		testutil.GitCmd(t, repo, "commit", "-m", "recent")
	}

	got, err := git.CommitCountSince(r, "main", 24*time.Hour)
	if err != nil {
		t.Fatalf("CommitCountSince: %v", err)
	}
	if got != 3 {
		t.Errorf("CommitCountSince = %d, want 3 (1 initial + 2 recent)", got)
	}
}

func TestCommitCountSinceExcludesOldCommits(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	// Backdate the single existing commit by 30 days (amend with old
	// committer+author date). With a 7-day since window, the result must
	// be 0: git's --since stops walking once it hits an older commit.
	oldDate := time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339)
	gitCmdWithCommitterDate(t, repo, oldDate, "commit", "--amend", "--no-edit")

	got, err := git.CommitCountSince(r, "main", 7*24*time.Hour)
	if err != nil {
		t.Fatalf("CommitCountSince: %v", err)
	}
	if got != 0 {
		t.Errorf("CommitCountSince over 7 days = %d, want 0 (only commit is 30 days old)", got)
	}
}

func TestCommitCountSinceUnknownBranch(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	_, err := git.CommitCountSince(r, "no-such-branch", 7*24*time.Hour)
	if err == nil {
		t.Fatal("CommitCountSince on unknown branch returned nil error, want non-nil")
	}
}

// gitCmdWithCommitterDate runs git with GIT_COMMITTER_DATE set, so that
// `--since` filtering (which looks at committer date) can be tested.
func gitCmdWithCommitterDate(t *testing.T, dir, date string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.email=test@test.com", "-c", "user.name=Test"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_COMMITTER_DATE="+date,
		"GIT_AUTHOR_DATE="+date,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
}
