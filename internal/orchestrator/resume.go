package orchestrator

import (
	"fmt"

	"github.com/joaovictor3g/agents/internal/state"
)

// resumeOutcome distinguishes an agent that was actually rebuilt from one that
// was already alive, so bulk resume can report an honest summary.
type resumeOutcome int

const (
	resumeStarted resumeOutcome = iota
	resumeAlreadyRunning
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
	_, err = o.resumeAgent(a, attach)
	return err
}

// ResumeAll walks the registry and resumes every agent, reusing the single-agent
// path. It recovers a whole machine after a reboot or `tmux kill-server`. Already
// running agents are skipped; individual failures are reported and stepped over
// so one broken agent never blocks the rest, and a summary lands at the end.
func (o *Orchestrator) ResumeAll() error {
	st, err := o.Store.Load()
	if err != nil {
		return err
	}
	if len(st.Agents) == 0 {
		o.UI.Info("No agents registered — nothing to resume.")
		return nil
	}

	// Recreate the session once, up front, so the per-agent restores never race
	// to create it (resumeAgent guards this too, but doing it here keeps the
	// bulk run's first window from creating a second, redundant session).
	if !o.Tmux.HasSession(o.Session) {
		if err := o.Tmux.NewSession(o.Session, o.Git.Root()); err != nil {
			return err
		}
	}

	var started, skipped int
	var failed []string
	for _, a := range st.Agents {
		outcome, err := o.resumeAgent(a, false)
		switch {
		case err != nil:
			o.UI.Error(fmt.Errorf("agent %q: %w", a.Name, err))
			failed = append(failed, a.Name)
		case outcome == resumeAlreadyRunning:
			skipped++
		default:
			started++
		}
	}

	o.UI.Info("")
	o.UI.Info("Resumed %d, %d already running, %d failed.", started, skipped, len(failed))
	if len(failed) > 0 {
		return fmt.Errorf("failed to resume %d of %d agents: %v", len(failed), len(st.Agents), failed)
	}
	return nil
}

// resumeAgent rebuilds one already-resolved agent. It is the shared core of both
// Resume and ResumeAll: restore a vanished worktree, ensure the session, then
// recreate the window and relaunch the provider unless the agent is already up.
func (o *Orchestrator) resumeAgent(a state.Agent, attach bool) (resumeOutcome, error) {
	// Restore the worktree if its directory vanished but the branch remains.
	if !o.FS.DirExists(a.Worktree) {
		branchExists, err := o.Git.BranchExists(a.Branch)
		if err != nil {
			return resumeStarted, err
		}
		if !branchExists {
			return resumeStarted, fmt.Errorf("agent %q cannot be resumed: branch %q no longer exists (run `agents delete %s` to clear it)", a.Name, a.Branch, a.Name)
		}
		if at, err := o.Git.BranchWorktree(a.Branch); err != nil {
			return resumeStarted, err
		} else if at != "" && at != a.Worktree {
			return resumeStarted, fmt.Errorf("agent %q: branch %q is checked out at %s (not %s)", a.Name, a.Branch, at, a.Worktree)
		} else if at == "" {
			if err := o.Git.AddWorktree(a.Worktree, a.Branch); err != nil {
				return resumeStarted, err
			}
			o.UI.Success("Restored worktree %s", o.UI.Dim(a.Worktree))
		}
	}

	if !o.Tmux.HasSession(o.Session) {
		if err := o.Tmux.NewSession(o.Session, o.Git.Root()); err != nil {
			return resumeStarted, err
		}
	}

	windows, err := o.Tmux.Windows(o.Session)
	if err != nil {
		return resumeStarted, err
	}
	if _, alive := windows[a.Name]; alive {
		o.UI.Info("Agent %s is already running.", o.UI.Bold(a.Name))
		if attach {
			return resumeAlreadyRunning, o.Tmux.Attach(o.Session, a.Name)
		}
		o.UI.Info("Run %s to join it.", o.UI.Bold("agents attach "+a.Name))
		return resumeAlreadyRunning, nil
	}

	prov, err := o.Providers.Resolve(a.Provider)
	if err != nil {
		return resumeStarted, err
	}
	// Resume launches the provider fresh (no prompt injection); the previous
	// AI conversation is gone, and providers expose their own resume flags.
	command, err := prov.CommandLine("")
	if err != nil {
		return resumeStarted, err
	}

	if err := o.Tmux.NewWindow(o.Session, a.Name, a.Worktree); err != nil {
		return resumeStarted, err
	}
	o.UI.Success("Recreated tmux window %s", o.UI.Bold(o.Session+":"+a.Name))

	if err := o.Tmux.SendCommand(o.Session, a.Name, command); err != nil {
		return resumeStarted, err
	}
	o.UI.Success("Restarted %s", a.Provider)

	o.UI.Info("")
	o.UI.Info("Agent resumed. Run %s to join it.", o.UI.Bold("agents attach "+a.Name))

	if attach {
		return resumeStarted, o.Tmux.Attach(o.Session, a.Name)
	}
	return resumeStarted, nil
}
