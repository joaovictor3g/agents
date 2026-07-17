package orchestrator

import "fmt"

// Problem is one health issue Doctor found. Agent is the owning agent's name,
// or "" for a repository-wide problem. Suggestion is a ready-to-run fix.
type Problem struct {
	Agent      string
	Message    string
	Suggestion string
}

// Diagnosis is the full result of `agents doctor`.
type Diagnosis struct {
	Problems []Problem
}

// Healthy reports whether no problems were found.
func (d *Diagnosis) Healthy() bool { return len(d.Problems) == 0 }

func (d *Diagnosis) add(agent, message, suggestion string) {
	d.Problems = append(d.Problems, Problem{Agent: agent, Message: message, Suggestion: suggestion})
}

// Doctor reconciles the registry against git, tmux, and the filesystem and
// reports every detected problem with an actionable fix. It never mutates
// anything: it is safe to run at any time and only reads state. Problems are
// ordered repo-wide first, then per agent in registry order, so the output is
// stable and scriptable.
func (o *Orchestrator) Doctor() (*Diagnosis, error) {
	d := &Diagnosis{}

	// Repo-wide checks on the main checkout: a detached HEAD or an unfinished
	// merge there blocks `agents merge` for every agent, so surface them once.
	current, err := o.Git.CurrentBranch()
	if err != nil {
		return nil, err
	}
	if current == "" {
		d.add("", "the main checkout is in detached HEAD state",
			"check out a branch, e.g. `git checkout "+o.defaultBranchHint()+"`")
	}
	if merging, err := o.Git.MergeInProgress(); err != nil {
		return nil, err
	} else if merging {
		d.add("", "a merge is in progress in the main checkout",
			"resolve it or run `git -C "+o.Git.Root()+" merge --abort`")
	}

	st, err := o.Store.Load()
	if err != nil {
		return nil, err
	}
	windows, err := o.Tmux.Windows(o.Session)
	if err != nil {
		return nil, err
	}

	for _, a := range st.Agents {
		// A missing worktree is terminal for this agent: the deeper git checks
		// below all need one, so classify it and move on. The branch decides
		// the fix — resume rebuilds a worktree for a live branch, delete clears
		// an agent whose branch is also gone (mirrors Resume's own logic).
		if !o.FS.DirExists(a.Worktree) {
			branchExists, err := o.Git.BranchExists(a.Branch)
			if err != nil {
				return nil, err
			}
			if branchExists {
				d.add(a.Name, "worktree directory is missing", "run `agents resume "+a.Name+"`")
			} else {
				d.add(a.Name, "worktree directory and branch "+a.Branch+" are both gone",
					"run `agents delete "+a.Name+"`")
			}
			continue
		}

		if _, alive := windows[a.Name]; !alive {
			d.add(a.Name, "tmux window is missing", "run `agents resume "+a.Name+"`")
		}

		// The provider binary can vanish after an agent is registered (uninstall,
		// PATH change), which would make a resume silently launch nothing.
		if prov, err := o.Providers.Resolve(a.Provider); err != nil {
			d.add(a.Name, err.Error(), "set a valid provider for this agent in your config")
		} else if o.LookPath(prov.Command) != nil {
			d.add(a.Name, fmt.Sprintf("provider command %q is not on PATH", prov.Command),
				"install "+prov.Command+" or fix your PATH")
		}

		detached, err := o.Git.DetachedHEAD(a.Worktree)
		if err != nil {
			return nil, err
		}
		if detached {
			d.add(a.Name, "worktree is in detached HEAD state",
				"run `git -C "+a.Worktree+" checkout "+a.Branch+"`")
		}

		merging, err := o.Git.MergeInProgressAt(a.Worktree)
		if err != nil {
			return nil, err
		}
		if merging {
			d.add(a.Name, "a merge is in progress in the worktree",
				"resolve it or run `git -C "+a.Worktree+" merge --abort`")
		}

		clean, err := o.Git.IsClean(a.Worktree)
		if err != nil {
			return nil, err
		}
		if !clean {
			d.add(a.Name, "worktree has uncommitted changes",
				"commit or stash them in "+a.Worktree)
		}
	}

	return d, nil
}

// defaultBranchHint returns the repository's default branch for use in a
// suggestion, falling back to a placeholder when it cannot be resolved so the
// hint never turns into a fatal error.
func (o *Orchestrator) defaultBranchHint() string {
	if b, err := o.Git.DefaultBranch(); err == nil && b != "" {
		return b
	}
	return "<branch>"
}
