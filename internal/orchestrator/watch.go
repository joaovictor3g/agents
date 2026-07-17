package orchestrator

import (
	"fmt"
	"time"
)

// WatchWindow is the reserved tmux window name for the dashboard. Agent names
// must start with an alphanumeric, so a leading underscore can never collide
// with an agent window. A leading '.' cannot be used here: tmux treats '.' as
// the window/pane separator in target specifiers, so a ".watch" window can be
// created but never targeted.
const WatchWindow = "_watch"

// tiledWarnThreshold is the point past which tiled panes get too small to be
// useful; watching still works, but we warn.
const tiledWarnThreshold = 9

// WatchOptions configures Watch.
type WatchOptions struct {
	Interval time.Duration
}

// Watch opens (or re-syncs) a read-only dashboard window showing every agent
// in a tiled grid of mirror panes, then focuses it. It reconciles desired
// panes (one per registered agent) against the panes already in the dashboard
// window — adding missing ones, killing stale ones — so re-running it after
// creating or deleting agents just updates the grid. Agents' real windows are
// never touched.
func (o *Orchestrator) Watch(opts WatchOptions) error {
	st, err := o.Store.Load()
	if err != nil {
		return err
	}
	if len(st.Agents) == 0 {
		return fmt.Errorf("no agents to watch (create one with `agents create <name>`)")
	}
	if len(st.Agents) > tiledWarnThreshold {
		o.UI.Warn("Watching %d agents — tiled panes will be small; consider `agents attach` for a single agent.", len(st.Agents))
	}

	desired := make(map[string]bool, len(st.Agents))
	order := make([]string, 0, len(st.Agents))
	for _, a := range st.Agents {
		desired[a.Name] = true
		order = append(order, a.Name)
	}

	panes, err := o.Tmux.PanesInWindow(o.Session, WatchWindow)
	if err != nil {
		return err
	}
	haveWindow := len(panes) > 0

	// paneByAgent maps agent name -> pane id for panes already present.
	paneByAgent := make(map[string]string, len(panes))
	for id, title := range panes {
		if desired[title] {
			paneByAgent[title] = id
		} else {
			if err := o.Tmux.KillPane(id); err != nil {
				return err
			}
		}
	}

	for _, name := range order {
		if _, ok := paneByAgent[name]; ok {
			continue
		}
		command := o.WatchPaneCommand(name, opts.Interval)

		var paneID string
		if !haveWindow {
			paneID, err = o.Tmux.NewWindowRunning(o.Session, WatchWindow, command)
			haveWindow = true
		} else {
			paneID, err = o.Tmux.SplitWindow(o.Session, WatchWindow, command)
		}
		if err != nil {
			return err
		}
		if err := o.Tmux.SetPaneTitle(paneID, name); err != nil {
			return err
		}
		// Re-tile after each split so the next split always has room.
		if err := o.Tmux.SelectLayout(o.Session, WatchWindow, "tiled"); err != nil {
			return err
		}
	}

	return o.Tmux.Attach(o.Session, WatchWindow)
}
