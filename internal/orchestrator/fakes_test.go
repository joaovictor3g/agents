package orchestrator

import (
	"bytes"
	"fmt"
	"time"

	"github.com/joaovictor3g/agents/internal/config"
	"github.com/joaovictor3g/agents/internal/git"
	"github.com/joaovictor3g/agents/internal/provider"
	"github.com/joaovictor3g/agents/internal/state"
	"github.com/joaovictor3g/agents/internal/ui"
)

type fakeGit struct {
	root          string
	currentBranch string
	defaultBranch string
	branches      map[string]bool
	checkedOutAt  map[string]string
	dirtyDirs     map[string]bool
	mergeConflict bool
	mergeInFlight bool
	log           []string
}

func newFakeGit() *fakeGit {
	return &fakeGit{
		root:          "/repo",
		currentBranch: "main",
		defaultBranch: "main",
		branches:      map[string]bool{"main": true},
		checkedOutAt:  map[string]string{},
		dirtyDirs:     map[string]bool{},
	}
}

func (g *fakeGit) record(format string, args ...any) {
	g.log = append(g.log, fmt.Sprintf(format, args...))
}

func (g *fakeGit) Root() string                           { return g.root }
func (g *fakeGit) CurrentBranch() (string, error)         { return g.currentBranch, nil }
func (g *fakeGit) DefaultBranch() (string, error)         { return g.defaultBranch, nil }
func (g *fakeGit) BranchExists(name string) (bool, error) { return g.branches[name], nil }

func (g *fakeGit) DeleteBranch(name string, force bool) error {
	g.record("delete-branch %s force=%v", name, force)
	delete(g.branches, name)
	return nil
}

func (g *fakeGit) AddWorktree(path, branch string) error {
	g.record("add-worktree %s %s", path, branch)
	g.checkedOutAt[branch] = path
	return nil
}

func (g *fakeGit) AddWorktreeNewBranch(path, branch, base string) error {
	g.record("add-worktree-new %s %s from %s", path, branch, base)
	g.branches[branch] = true
	g.checkedOutAt[branch] = path
	return nil
}

func (g *fakeGit) RemoveWorktree(path string, force bool) error {
	if g.dirtyDirs[path] && !force {
		return fmt.Errorf("worktree is dirty")
	}
	g.record("remove-worktree %s force=%v", path, force)
	return nil
}

func (g *fakeGit) BranchWorktree(branch string) (string, error) { return g.checkedOutAt[branch], nil }
func (g *fakeGit) IsClean(dir string) (bool, error)             { return !g.dirtyDirs[dir], nil }

func (g *fakeGit) Checkout(branch string) error {
	g.record("checkout %s", branch)
	g.currentBranch = branch
	return nil
}

func (g *fakeGit) Merge(branch string) error {
	if g.mergeConflict {
		g.mergeInFlight = true
		return fmt.Errorf("%w: CONFLICT", git.ErrMergeConflict)
	}
	g.record("merge %s", branch)
	return nil
}

func (g *fakeGit) MergeInProgress() (bool, error) { return g.mergeInFlight, nil }

type fakePane struct {
	window string
	title  string
}

type fakeTmux struct {
	sessions map[string]bool
	windows  map[string]string
	sent     map[string]string
	panes    map[string]*fakePane // pane id -> pane
	nextPane int
	layouts  []string
	attached string
	log      []string
}

func newFakeTmux() *fakeTmux {
	return &fakeTmux{
		sessions: map[string]bool{},
		windows:  map[string]string{},
		sent:     map[string]string{},
		panes:    map[string]*fakePane{},
	}
}

func (t *fakeTmux) PanesInWindow(session, window string) (map[string]string, error) {
	out := map[string]string{}
	for id, p := range t.panes {
		if p.window == window {
			out[id] = p.title
		}
	}
	return out, nil
}

func (t *fakeTmux) NewWindowRunning(session, window, command string) (string, error) {
	t.windows[window] = "agents"
	t.nextPane++
	id := fmt.Sprintf("%%%d", t.nextPane)
	t.panes[id] = &fakePane{window: window}
	t.log = append(t.log, "new-window-running "+window)
	return id, nil
}

func (t *fakeTmux) SplitWindow(session, window, command string) (string, error) {
	t.nextPane++
	id := fmt.Sprintf("%%%d", t.nextPane)
	t.panes[id] = &fakePane{window: window}
	t.log = append(t.log, "split "+window)
	return id, nil
}

func (t *fakeTmux) KillPane(paneID string) error {
	delete(t.panes, paneID)
	t.log = append(t.log, "kill-pane "+paneID)
	return nil
}

func (t *fakeTmux) SetPaneTitle(paneID, title string) error {
	if p, ok := t.panes[paneID]; ok {
		p.title = title
	}
	return nil
}

func (t *fakeTmux) SelectLayout(session, window, layout string) error {
	t.layouts = append(t.layouts, layout)
	return nil
}

func (t *fakeTmux) HasSession(name string) bool { return t.sessions[name] }

func (t *fakeTmux) NewSession(name, dir string) error {
	t.sessions[name] = true
	return nil
}

func (t *fakeTmux) NewWindow(session, name, dir string) error {
	t.windows[name] = "zsh"
	t.log = append(t.log, "new-window "+name+" in "+dir)
	return nil
}

func (t *fakeTmux) KillWindow(session, name string) error {
	delete(t.windows, name)
	t.log = append(t.log, "kill-window "+name)
	return nil
}

func (t *fakeTmux) Windows(session string) (map[string]string, error) {
	out := make(map[string]string, len(t.windows))
	for k, v := range t.windows {
		out[k] = v
	}
	return out, nil
}

func (t *fakeTmux) SendCommand(session, window, command string) error {
	t.sent[window] = command
	return nil
}

func (t *fakeTmux) Attach(session, window string) error {
	t.attached = session + ":" + window
	return nil
}

type fakeStore struct {
	state state.State
	saves int
}

func (s *fakeStore) Load() (*state.State, error) {
	copied := state.State{Agents: append([]state.Agent(nil), s.state.Agents...)}
	return &copied, nil
}

func (s *fakeStore) Save(st *state.State) error {
	s.state = state.State{Agents: append([]state.Agent(nil), st.Agents...)}
	s.saves++
	return nil
}

type fakeFS struct{ missing map[string]bool }

func (f fakeFS) DirExists(path string) bool { return !f.missing[path] }

type fakeNotifier struct{ messages []string }

func (n *fakeNotifier) Notify(title, message string) {
	n.messages = append(n.messages, message)
}

type world struct {
	git      *fakeGit
	tmux     *fakeTmux
	store    *fakeStore
	fs       *fakeFS
	notifier *fakeNotifier
	orch     *Orchestrator
	out      *bytes.Buffer
}

func newWorld() *world {
	w := &world{
		git:      newFakeGit(),
		tmux:     newFakeTmux(),
		store:    &fakeStore{},
		fs:       &fakeFS{missing: map[string]bool{}},
		notifier: &fakeNotifier{},
		out:      &bytes.Buffer{},
	}
	cfg := config.Default()
	w.orch = &Orchestrator{
		Git:              w.git,
		Tmux:             w.tmux,
		Store:            w.store,
		FS:               w.fs,
		Cfg:              cfg,
		Providers:        provider.NewRegistry(cfg),
		UI:               ui.NewFor(w.out, w.out, false),
		Notifier:         w.notifier,
		Session:          "repo",
		ExcludeWorktrees: func() error { return nil },
		WorktreePath:     func(name string) string { return "/repo/worktrees/" + name },
		WatchPaneCommand: func(name string, interval time.Duration) string {
			return "watch-pane " + name
		},
	}
	return w
}
