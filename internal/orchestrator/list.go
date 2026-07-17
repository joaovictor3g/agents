package orchestrator

import (
	"github.com/joaovictor3g/agents/internal/state"
)

// AgentInfo is one reconciled row of `agents list`.
type AgentInfo struct {
	Agent  state.Agent
	Status Status
}

// List returns every registered agent with its reconciled status.
func (o *Orchestrator) List() ([]AgentInfo, error) {
	st, err := o.Store.Load()
	if err != nil {
		return nil, err
	}
	windows, err := o.Tmux.Windows(o.Session)
	if err != nil {
		return nil, err
	}

	infos := make([]AgentInfo, 0, len(st.Agents))
	for _, a := range st.Agents {
		paneCmd, windowExists := windows[a.Name]
		infos = append(infos, AgentInfo{
			Agent:  a,
			Status: statusOf(o.FS.DirExists(a.Worktree), windowExists, paneCmd),
		})
	}
	return infos, nil
}

// Attach focuses the agent's tmux window.
func (o *Orchestrator) Attach(name string) error {
	_, a, err := o.agent(name)
	if err != nil {
		return err
	}
	return o.Tmux.Attach(o.Session, a.Name)
}
