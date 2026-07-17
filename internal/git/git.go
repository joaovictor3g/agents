// Package git wraps the git CLI. It is a thin translation layer; all policy
// (guards, ordering, defaults) lives in the orchestrator.
package git

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joaovictor3g/agents/internal/execx"
)

// ErrMergeConflict is returned by Merge when the merge stopped on conflicts
// and was left in progress for manual resolution.
var ErrMergeConflict = errors.New("merge conflict")

// Client runs git commands against one repository.
type Client struct {
	run  execx.Runner
	root string
}

// Discover locates the repository containing dir.
func Discover(run execx.Runner, dir string) (*Client, error) {
	out, err := run.Output("git", "-C", dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("not inside a git repository (run agents from within a repo)")
	}
	return &Client{run: run, root: out}, nil
}

// Root returns the repository root of the current checkout.
func (c *Client) Root() string { return c.root }

func (c *Client) git(args ...string) (string, error) {
	return c.run.Output("git", append([]string{"-C", c.root}, args...)...)
}

// CommonDir returns the absolute path of the shared .git directory,
// which is the same for every worktree of the repository.
func (c *Client) CommonDir() (string, error) {
	out, err := c.git("rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(c.root, out)
	}
	return filepath.Clean(out), nil
}

// CurrentBranch returns the checked-out branch, or "" if HEAD is detached.
func (c *Client) CurrentBranch() (string, error) {
	out, err := c.git("symbolic-ref", "--short", "-q", "HEAD")
	if err != nil {
		if execx.ExitCode(err) == 1 {
			return "", nil
		}
		return "", err
	}
	return out, nil
}

// DefaultBranch resolves the repository's default branch: origin/HEAD when
// set, otherwise a local main or master branch.
func (c *Client) DefaultBranch() (string, error) {
	out, err := c.git("symbolic-ref", "--short", "-q", "refs/remotes/origin/HEAD")
	if err == nil {
		return strings.TrimPrefix(out, "origin/"), nil
	}
	for _, name := range []string{"main", "master"} {
		exists, err := c.BranchExists(name)
		if err != nil {
			return "", err
		}
		if exists {
			return name, nil
		}
	}
	return "", fmt.Errorf("cannot determine the default branch (no origin/HEAD, main, or master)")
}

// BranchExists reports whether a local branch exists.
func (c *Client) BranchExists(name string) (bool, error) {
	_, err := c.git("show-ref", "--verify", "--quiet", "refs/heads/"+name)
	if err != nil {
		if execx.ExitCode(err) == 1 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// DeleteBranch removes a local branch; force uses -D to discard unmerged work.
func (c *Client) DeleteBranch(name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := c.git("branch", flag, name)
	return err
}

// AddWorktree checks out an existing branch into a new worktree.
func (c *Client) AddWorktree(path, branch string) error {
	_, err := c.git("worktree", "add", path, branch)
	return err
}

// AddWorktreeNewBranch creates a branch from base and checks it out into a
// new worktree.
func (c *Client) AddWorktreeNewBranch(path, branch, base string) error {
	_, err := c.git("worktree", "add", "-b", branch, path, base)
	return err
}

// RemoveWorktree removes a worktree; force discards uncommitted changes.
func (c *Client) RemoveWorktree(path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	_, err := c.git(append(args, path)...)
	return err
}

// BranchWorktree returns the path of the worktree where branch is checked
// out, or "" if it is not checked out anywhere.
func (c *Client) BranchWorktree(branch string) (string, error) {
	out, err := c.git("worktree", "list", "--porcelain")
	if err != nil {
		return "", err
	}
	var path string
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			path = strings.TrimPrefix(line, "worktree ")
		case line == "branch refs/heads/"+branch:
			return path, nil
		}
	}
	return "", nil
}

// IsClean reports whether the working tree at dir has no uncommitted changes.
func (c *Client) IsClean(dir string) (bool, error) {
	out, err := c.run.Output("git", "-C", dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out == "", nil
}

// Checkout switches the main checkout to branch.
func (c *Client) Checkout(branch string) error {
	_, err := c.git("checkout", branch)
	return err
}

// Merge merges branch into the current branch. On conflict the merge is left
// in progress and ErrMergeConflict is returned.
func (c *Client) Merge(branch string) error {
	_, err := c.git("merge", branch)
	if err != nil {
		if inProgress, mErr := c.MergeInProgress(); mErr == nil && inProgress {
			return fmt.Errorf("%w: %s", ErrMergeConflict, execx.Stderr(err))
		}
		return err
	}
	return nil
}

// MergeInProgress reports whether the main checkout has an unfinished merge.
func (c *Client) MergeInProgress() (bool, error) {
	gitDir, err := c.git("rev-parse", "--git-dir")
	if err != nil {
		return false, err
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(c.root, gitDir)
	}
	_, err = os.Stat(filepath.Join(gitDir, "MERGE_HEAD"))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
