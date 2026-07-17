package orchestrator

import (
	"fmt"
)

// DeleteOptions configures Delete.
type DeleteOptions struct {
	// DeleteBranch also removes the branch; by default it is kept because it
	// may hold the agent's only unmerged work.
	DeleteBranch bool
	// Force discards uncommitted worktree changes and unmerged branches.
	Force bool
}

// Delete tears an agent down. It is idempotent: pieces the user already
// removed by hand are treated as done, so a half-broken agent can always be
// cleaned up. The registry entry is removed last so a failed step leaves the
// agent visible in `agents list` rather than orphaned.
func (o *Orchestrator) Delete(name string, opts DeleteOptions) error {
	st, a, err := o.agent(name)
	if err != nil {
		return err
	}

	windows, err := o.Tmux.Windows(o.Session)
	if err != nil {
		return err
	}
	if _, exists := windows[a.Name]; exists {
		if err := o.Tmux.KillWindow(o.Session, a.Name); err != nil {
			return err
		}
		o.UI.Success("Stopped session %s", o.UI.Bold(o.Session+":"+a.Name))
	}

	if o.FS.DirExists(a.Worktree) {
		if err := o.Git.RemoveWorktree(a.Worktree, opts.Force); err != nil {
			return fmt.Errorf("worktree %s has uncommitted changes (commit them or use --force): %w", a.Worktree, err)
		}
		o.UI.Success("Removed worktree %s", o.UI.Dim(a.Worktree))
	}

	if opts.DeleteBranch {
		exists, err := o.Git.BranchExists(a.Branch)
		if err != nil {
			return err
		}
		if exists {
			if err := o.Git.DeleteBranch(a.Branch, opts.Force); err != nil {
				return fmt.Errorf("branch %s is not merged (merge it or use --force): %w", a.Branch, err)
			}
			o.UI.Success("Deleted branch %s", o.UI.Bold(a.Branch))
		}
	} else {
		o.UI.Info("Kept branch %s (delete it with `agents delete %s --branch` next time, or `git branch -d %s`)",
			o.UI.Bold(a.Branch), a.Name, a.Branch)
	}

	st.Remove(a.Name)
	return o.Store.Save(st)
}
