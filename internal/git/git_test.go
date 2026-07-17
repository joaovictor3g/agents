package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/victordias/agents/internal/execx"
)

// These tests run against a real git binary in a temp repository.

func setupRepo(t *testing.T) *Client {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	run := execx.System{}
	mustGit := func(args ...string) {
		t.Helper()
		if _, err := run.Output("git", append([]string{"-C", dir}, args...)...); err != nil {
			t.Fatal(err)
		}
	}
	mustGit("init", "-b", "main")
	mustGit("config", "user.email", "test@example.com")
	mustGit("config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit("add", ".")
	mustGit("commit", "-m", "initial")

	c, err := Discover(run, dir)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestDiscoverOutsideRepoFails(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	if _, err := Discover(execx.System{}, t.TempDir()); err == nil {
		t.Fatal("expected error outside a repository")
	}
}

func TestBranchAndWorktreeLifecycle(t *testing.T) {
	c := setupRepo(t)

	exists, err := c.BranchExists("auth")
	if err != nil || exists {
		t.Fatalf("BranchExists(auth) = %v, %v", exists, err)
	}

	wt := filepath.Join(c.Root(), "worktrees", "auth")
	if err := c.AddWorktreeNewBranch(wt, "auth", "main"); err != nil {
		t.Fatal(err)
	}

	exists, err = c.BranchExists("auth")
	if err != nil || !exists {
		t.Fatalf("BranchExists(auth) after create = %v, %v", exists, err)
	}

	at, err := c.BranchWorktree("auth")
	if err != nil {
		t.Fatal(err)
	}
	if at != wt {
		t.Fatalf("BranchWorktree = %q, want %q", at, wt)
	}

	clean, err := c.IsClean(wt)
	if err != nil || !clean {
		t.Fatalf("IsClean = %v, %v", clean, err)
	}
	if err := os.WriteFile(filepath.Join(wt, "new.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	clean, err = c.IsClean(wt)
	if err != nil || clean {
		t.Fatalf("IsClean after change = %v, %v", clean, err)
	}

	if err := c.RemoveWorktree(wt, false); err == nil {
		t.Fatal("removing dirty worktree without force should fail")
	}
	if err := c.RemoveWorktree(wt, true); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteBranch("auth", false); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultBranchFallsBackToMain(t *testing.T) {
	c := setupRepo(t)
	got, err := c.DefaultBranch()
	if err != nil {
		t.Fatal(err)
	}
	if got != "main" {
		t.Fatalf("DefaultBranch = %q", got)
	}
}

func TestMergeConflictDetection(t *testing.T) {
	c := setupRepo(t)
	run := execx.System{}
	mustGit := func(args ...string) {
		t.Helper()
		if _, err := run.Output("git", append([]string{"-C", c.Root()}, args...)...); err != nil {
			t.Fatal(err)
		}
	}

	wt := filepath.Join(c.Root(), "worktrees", "feature")
	if err := c.AddWorktreeNewBranch(wt, "feature", "main"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, "README.md"), []byte("feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := run.Output("git", "-C", wt, "commit", "-am", "feature change"); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(c.Root(), "README.md"), []byte("main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit("commit", "-am", "main change")

	err := c.Merge("feature")
	if !errors.Is(err, ErrMergeConflict) {
		t.Fatalf("err = %v, want ErrMergeConflict", err)
	}
	inProgress, err := c.MergeInProgress()
	if err != nil || !inProgress {
		t.Fatalf("MergeInProgress = %v, %v", inProgress, err)
	}

	mustGit("merge", "--abort")

	if cur, _ := c.CurrentBranch(); cur != "main" {
		t.Fatalf("CurrentBranch = %q", cur)
	}
}

func TestCurrentBranchDetached(t *testing.T) {
	c := setupRepo(t)
	run := execx.System{}
	if _, err := run.Output("git", "-C", c.Root(), "checkout", "--detach"); err != nil {
		t.Fatal(err)
	}
	got, err := c.CurrentBranch()
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("CurrentBranch on detached HEAD = %q, want empty", got)
	}
}
