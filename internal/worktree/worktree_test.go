package worktree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPath(t *testing.T) {
	if got := Path("/repo", "worktrees", "auth"); got != "/repo/worktrees/auth" {
		t.Errorf("relative root: got %q", got)
	}
	if got := Path("/repo", "/elsewhere/wt", "auth"); got != "/elsewhere/wt/auth" {
		t.Errorf("absolute root: got %q", got)
	}
}

func TestInsideRepo(t *testing.T) {
	if !InsideRepo("/repo", "worktrees") {
		t.Error("relative root should be inside repo")
	}
	if InsideRepo("/repo", "../siblings") {
		t.Error("parent-relative root should be outside repo")
	}
	if InsideRepo("/repo", "/elsewhere") {
		t.Error("absolute foreign root should be outside repo")
	}
	if !InsideRepo("/repo", "/repo/worktrees") {
		t.Error("absolute root under repo should be inside")
	}
}

func TestEnsureExcludedIdempotent(t *testing.T) {
	commonDir := t.TempDir()
	for range 3 {
		if err := EnsureExcluded(commonDir, "/repo", "worktrees"); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(commonDir, "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), "/worktrees/"); got != 1 {
		t.Fatalf("exclude line appears %d times:\n%s", got, data)
	}
}

func TestEnsureExcludedSkipsOutsideRepo(t *testing.T) {
	commonDir := t.TempDir()
	if err := EnsureExcluded(commonDir, "/repo", "/elsewhere/wt"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(commonDir, "info", "exclude")); !os.IsNotExist(err) {
		t.Fatal("exclude file should not be created for out-of-repo roots")
	}
}

func TestEnsureExcludedPreservesExistingContent(t *testing.T) {
	commonDir := t.TempDir()
	infoDir := filepath.Join(commonDir, "info")
	os.MkdirAll(infoDir, 0o755)
	os.WriteFile(filepath.Join(infoDir, "exclude"), []byte("*.log"), 0o644)

	if err := EnsureExcluded(commonDir, "/repo", "worktrees"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(infoDir, "exclude"))
	content := string(data)
	if !strings.Contains(content, "*.log") || !strings.Contains(content, "/worktrees/") {
		t.Fatalf("content mangled:\n%s", content)
	}
}
