// Package worktree decides where agent worktrees live and keeps in-repo
// worktree roots out of git status via .git/info/exclude — never by touching
// the user's tracked .gitignore.
package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Path returns the absolute worktree path for an agent. root is taken from
// configuration and may be absolute or relative to the repo root.
func Path(repoRoot, root, name string) string {
	if filepath.IsAbs(root) {
		return filepath.Join(root, name)
	}
	return filepath.Join(repoRoot, root, name)
}

// InsideRepo reports whether the configured worktree root lives inside the
// repository.
func InsideRepo(repoRoot, root string) bool {
	if !filepath.IsAbs(root) {
		return !strings.HasPrefix(filepath.Clean(root), "..")
	}
	rel, err := filepath.Rel(repoRoot, root)
	return err == nil && !strings.HasPrefix(rel, "..")
}

// EnsureExcluded idempotently adds the worktree root to .git/info/exclude
// when it lives inside the repository.
func EnsureExcluded(commonDir, repoRoot, root string) error {
	if !InsideRepo(repoRoot, root) {
		return nil
	}
	rel := root
	if filepath.IsAbs(root) {
		var err error
		rel, err = filepath.Rel(repoRoot, root)
		if err != nil {
			return err
		}
	}
	line := "/" + filepath.ToSlash(filepath.Clean(rel)) + "/"

	excludePath := filepath.Join(commonDir, "info", "exclude")
	data, err := os.ReadFile(excludePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading %s: %w", excludePath, err)
	}
	for _, existing := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(existing) == line {
			return nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return err
	}
	content := string(data)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += line + "\n"
	if err := os.WriteFile(excludePath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("updating %s: %w", excludePath, err)
	}
	return nil
}
