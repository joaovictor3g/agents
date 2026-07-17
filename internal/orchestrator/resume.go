package orchestrator

import (
	"fmt"
)

// Resume brings a stopped agent back to life. A reboot (or `tmux kill-server`)
// destroys the tmux window and the provider process, but the branch, worktree,
// and registry entry all survive. Resume rebuilds only the missing pieces:
// re-adds the worktree if its directory is gone, recreates the tmux window, and
// relaunches the provider. It is idempotent — an agent whose window is already
// alive is simply re-attached, never duplicated. No new branch is ever created.
func (o *Orchestrator) Resume(name string, attach bool) error {
	_, a, err := o.agent(name)
	if err != nil {
		return err
	}

	// Restore the worktree if its directory vanished but the branch remains.
	if !o.FS.DirExists(a.Worktree) {
		branchExists, err := o.Git.BranchExists(a.Branch)
		if err != nil {
			return err
		}
		if !branchExists {
			return fmt.Errorf("agent %q cannot be resumed: branch %q no longer exists (run `agents delete %s` to clear it)", name, a.Branch, name)
		}
		if at, err := o.Git.BranchWorktree(a.Branch); err != nil {
			return err
		} else if at != "" && at != a.Worktree {
			return fmt.Errorf("agent %q: branch %q is checked out at %s (not %s)", name, a.Branch, at, a.Worktree)
		} else if at == "" {
			if err := o.Git.AddWorktree(a.Worktree, a.Branch); err != nil {
				return err
			}
			o.UI.Success("Restored worktree %s", o.UI.Dim(a.Worktree))
		}
	}

	if !o.Tmux.HasSession(o.Session) {
		if err := o.Tmux.NewSession(o.Session, o.Git.Root()); err != nil {
			return err
		}
	}

	windows, err := o.Tmux.Windows(o.Session)
	if err != nil {
		return err
	}
	if _, alive := windows[a.Name]; alive {
		o.UI.Info("Agent %s is already running.", o.UI.Bold(name))
		if attach {
			return o.Tmux.Attach(o.Session, a.Name)
		}
		o.UI.Info("Run %s to join it.", o.UI.Bold("agents attach "+name))
		return nil
	}

	prov, err := o.Providers.Resolve(a.Provider)
	if err != nil {
		return err
	}
	// Resume launches the provider fresh (no prompt injection); the previous
	// AI conversation is gone, and providers expose their own resume flags.
	command, err := prov.CommandLine("")
	if err != nil {
		return err
	}

	if err := o.Tmux.NewWindow(o.Session, a.Name, a.Worktree); err != nil {
		return err
	}
	o.UI.Success("Recreated tmux window %s", o.UI.Bold(o.Session+":"+a.Name))

	if err := o.Tmux.SendCommand(o.Session, a.Name, command); err != nil {
		return err
	}
	o.UI.Success("Restarted %s", a.Provider)

	o.UI.Info("")
	o.UI.Info("Agent resumed. Run %s to join it.", o.UI.Bold("agents attach "+name))

	if attach {
		return o.Tmux.Attach(o.Session, a.Name)
	}
	return nil
}
