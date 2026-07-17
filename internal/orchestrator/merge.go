package orchestrator

import (
	"errors"
	"fmt"

	"github.com/joaovictor3g/agents/internal/git"
)

// MergeOptions configures Merge.
type MergeOptions struct {
	// KeepBranch skips deleting the merged branch.
	KeepBranch bool
	// Force merges even when the agent worktree has uncommitted changes.
	Force bool
}

// Merge integrates an agent's branch into the default branch, then tears the
// agent down. Preflight guards abort before anything is touched; a conflict
// leaves the merge in progress with zero teardown so nothing is lost.
func (o *Orchestrator) Merge(name string, opts MergeOptions) error {
	st, a, err := o.agent(name)
	if err != nil {
		return err
	}

	defaultBranch, err := o.Git.DefaultBranch()
	if err != nil {
		return err
	}

	if inProgress, err := o.Git.MergeInProgress(); err != nil {
		return err
	} else if inProgress {
		return fmt.Errorf("a merge is already in progress in %s — resolve it first", o.Git.Root())
	}

	if clean, err := o.Git.IsClean(o.Git.Root()); err != nil {
		return err
	} else if !clean {
		return fmt.Errorf("the main checkout at %s has uncommitted changes — commit or stash them first", o.Git.Root())
	}

	if o.FS.DirExists(a.Worktree) && !opts.Force {
		if clean, err := o.Git.IsClean(a.Worktree); err != nil {
			return err
		} else if !clean {
			return fmt.Errorf("agent %q has uncommitted changes in %s — commit them, or pass --force to merge without them", name, a.Worktree)
		}
	}

	current, err := o.Git.CurrentBranch()
	if err != nil {
		return err
	}
	if current != defaultBranch {
		if err := o.Git.Checkout(defaultBranch); err != nil {
			return err
		}
		o.UI.Success("Checked out %s", o.UI.Bold(defaultBranch))
	}

	if err := o.Git.Merge(a.Branch); err != nil {
		if errors.Is(err, git.ErrMergeConflict) {
			o.UI.Warn("Merge of %s stopped on conflicts.", o.UI.Bold(a.Branch))
			o.UI.Info("The merge was left in progress in %s.", o.Git.Root())
			o.UI.Info("Resolve the conflicts and commit, then run %s.", o.UI.Bold("agents delete "+name))
			o.UI.Info("Nothing was deleted: the agent, worktree, and branch are untouched.")
			return fmt.Errorf("merge conflict in %s", a.Branch)
		}
		return err
	}
	o.UI.Success("Merged %s into %s", o.UI.Bold(a.Branch), o.UI.Bold(defaultBranch))

	windows, err := o.Tmux.Windows(o.Session)
	if err != nil {
		return err
	}
	if _, exists := windows[a.Name]; exists {
		if err := o.Tmux.KillWindow(o.Session, a.Name); err != nil {
			return err
		}
	}
	if o.FS.DirExists(a.Worktree) {
		if err := o.Git.RemoveWorktree(a.Worktree, opts.Force); err != nil {
			return err
		}
		o.UI.Success("Removed worktree %s", o.UI.Dim(a.Worktree))
	}

	if !opts.KeepBranch {
		if err := o.Git.DeleteBranch(a.Branch, false); err != nil {
			return err
		}
		o.UI.Success("Deleted branch %s", o.UI.Bold(a.Branch))
	}

	st.Remove(a.Name)
	if err := o.Store.Save(st); err != nil {
		return err
	}

	if o.Cfg.Notifications {
		o.Notifier.Notify("agents", fmt.Sprintf("Merged %s into %s", a.Branch, defaultBranch))
	}
	return nil
}
