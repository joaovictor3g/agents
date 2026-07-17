package orchestrator

import (
	"strings"
	"testing"

	"github.com/joaovictor3g/agents/internal/state"
)

func paneTitles(t *fakeTmux) map[string]bool {
	titles := map[string]bool{}
	for _, p := range t.panes {
		if p.window == WatchWindow {
			titles[p.title] = true
		}
	}
	return titles
}

func TestWatchNoAgentsFails(t *testing.T) {
	w := newWorld()
	err := w.orch.Watch(WatchOptions{})
	if err == nil || !strings.Contains(err.Error(), "no agents") {
		t.Fatalf("err = %v", err)
	}
}

func TestWatchCreatesOnePanePerAgent(t *testing.T) {
	w := newWorld()
	w.store.state.Add(state.Agent{Name: "auth"})
	w.store.state.Add(state.Agent{Name: "tests"})
	w.store.state.Add(state.Agent{Name: "review"})

	if err := w.orch.Watch(WatchOptions{}); err != nil {
		t.Fatal(err)
	}

	titles := paneTitles(w.tmux)
	for _, name := range []string{"auth", "tests", "review"} {
		if !titles[name] {
			t.Errorf("missing pane for %s; have %v", name, titles)
		}
	}
	if len(titles) != 3 {
		t.Errorf("want 3 panes, got %d", len(titles))
	}
	if w.tmux.attached != "repo:"+WatchWindow {
		t.Errorf("attached = %q", w.tmux.attached)
	}
	// First agent opens the window; the rest split it.
	if w.tmux.log[0] != "new-window-running "+WatchWindow {
		t.Errorf("first op = %q, want new-window-running", w.tmux.log[0])
	}
}

func TestWatchReconcilesAddsAndRemoves(t *testing.T) {
	w := newWorld()
	w.store.state.Add(state.Agent{Name: "auth"})
	w.store.state.Add(state.Agent{Name: "stale"})
	if err := w.orch.Watch(WatchOptions{}); err != nil {
		t.Fatal(err)
	}

	// "stale" goes away, "new" appears.
	w.store.state.Remove("stale")
	w.store.state.Add(state.Agent{Name: "new"})

	if err := w.orch.Watch(WatchOptions{}); err != nil {
		t.Fatal(err)
	}

	titles := paneTitles(w.tmux)
	if titles["stale"] {
		t.Error("stale pane should have been killed")
	}
	if !titles["auth"] || !titles["new"] {
		t.Errorf("want auth and new panes, have %v", titles)
	}
	if len(titles) != 2 {
		t.Errorf("want 2 panes, got %d", len(titles))
	}
}

func TestWatchIsIdempotent(t *testing.T) {
	w := newWorld()
	w.store.state.Add(state.Agent{Name: "auth"})
	w.store.state.Add(state.Agent{Name: "tests"})

	if err := w.orch.Watch(WatchOptions{}); err != nil {
		t.Fatal(err)
	}
	firstCount := len(w.tmux.panes)

	if err := w.orch.Watch(WatchOptions{}); err != nil {
		t.Fatal(err)
	}
	if len(w.tmux.panes) != firstCount {
		t.Errorf("re-running watch changed pane count: %d -> %d", firstCount, len(w.tmux.panes))
	}
}
