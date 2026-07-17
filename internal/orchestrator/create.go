package orchestrator

import (
	"fmt"
	"time"

	"github.com/joaovictor3g/agents/internal/state"
	"github.com/joaovictor3g/agents/internal/templates"
)

// CreateOptions configures Create.
type CreateOptions struct {
	Name     string
	Provider string
	Template string
	Prompt   string
	Base     string
	Attach   bool
}

// Create provisions a complete agent: branch, worktree, tmux window, and the
// provider session. The registry entry is saved before the tmux steps so a
// partially created agent can always be cleaned up with `agents delete`.
func (o *Orchestrator) Create(opts CreateOptions) error {
	if err := ValidateName(opts.Name); err != nil {
		return err
	}

	st, err := o.Store.Load()
	if err != nil {
		return err
	}
	if _, exists := st.Get(opts.Name); exists {
		return fmt.Errorf("agent %q already exists (run `agents attach %s`)", opts.Name, opts.Name)
	}

	providerName := opts.Provider
	if providerName == "" {
		providerName = o.Cfg.DefaultProvider
	}
	prov, err := o.Providers.Resolve(providerName)
	if err != nil {
		return err
	}

	prompt := opts.Prompt
	if opts.Template != "" {
		prompt, err = templates.Resolve(opts.Template, o.Cfg.TemplateDirs)
		if err != nil {
			return err
		}
	}
	command, err := prov.CommandLine(prompt)
	if err != nil {
		return err
	}

	branchExists, err := o.Git.BranchExists(opts.Name)
	if err != nil {
		return err
	}
	if branchExists {
		if at, err := o.Git.BranchWorktree(opts.Name); err != nil {
			return err
		} else if at != "" {
			return fmt.Errorf("branch %q is already checked out at %s", opts.Name, at)
		}
	}

	base := opts.Base
	if base == "" && !branchExists {
		base, err = o.Git.DefaultBranch()
		if err != nil {
			return fmt.Errorf("%w (use --base to pick a starting point)", err)
		}
	}

	wtPath := o.WorktreePath(opts.Name)
	if branchExists {
		if err := o.Git.AddWorktree(wtPath, opts.Name); err != nil {
			return err
		}
		o.UI.Success("Reusing branch %s", o.UI.Bold(opts.Name))
	} else {
		if err := o.Git.AddWorktreeNewBranch(wtPath, opts.Name, base); err != nil {
			return err
		}
		o.UI.Success("Created branch %s (from %s)", o.UI.Bold(opts.Name), base)
	}
	o.UI.Success("Created worktree %s", o.UI.Dim(wtPath))

	if err := o.ExcludeWorktrees(); err != nil {
		return err
	}

	st.Add(state.Agent{
		Name:      opts.Name,
		Provider:  providerName,
		Branch:    opts.Name,
		Worktree:  wtPath,
		CreatedAt: time.Now().UTC(),
	})
	if err := o.Store.Save(st); err != nil {
		return err
	}

	if !o.Tmux.HasSession(o.Session) {
		if err := o.Tmux.NewSession(o.Session, o.Git.Root()); err != nil {
			return cleanupHint(opts.Name, err)
		}
	}
	if err := o.Tmux.NewWindow(o.Session, opts.Name, wtPath); err != nil {
		return cleanupHint(opts.Name, err)
	}
	o.UI.Success("Created tmux window %s", o.UI.Bold(o.Session+":"+opts.Name))

	if err := o.Tmux.SendCommand(o.Session, opts.Name, command); err != nil {
		return cleanupHint(opts.Name, err)
	}
	o.UI.Success("Started %s", providerName)

	o.UI.Info("")
	o.UI.Info("Agent ready. Run %s to join it.", o.UI.Bold("agents attach "+opts.Name))
	if o.Cfg.Notifications {
		o.Notifier.Notify("agents", fmt.Sprintf("Agent %s is ready", opts.Name))
	}

	if opts.Attach {
		return o.Tmux.Attach(o.Session, opts.Name)
	}
	return nil
}

func cleanupHint(name string, err error) error {
	return fmt.Errorf("%w (run `agents delete %s` to clean up)", err, name)
}
