// Package orchestrator implements the agents use-cases. It owns every policy
// decision — preflight guards, teardown ordering, idempotence, status
// reconciliation — and talks to git, tmux, and the registry only through
// interfaces so all of it is testable with fakes.
package orchestrator

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/joaovictor3g/agents/internal/config"
	"github.com/joaovictor3g/agents/internal/provider"
	"github.com/joaovictor3g/agents/internal/state"
	"github.com/joaovictor3g/agents/internal/ui"
)

// GitClient is the git surface the orchestrator needs.
type GitClient interface {
	Root() string
	CurrentBranch() (string, error)
	DefaultBranch() (string, error)
	BranchExists(name string) (bool, error)
	DeleteBranch(name string, force bool) error
	AddWorktree(path, branch string) error
	AddWorktreeNewBranch(path, branch, base string) error
	RemoveWorktree(path string, force bool) error
	BranchWorktree(branch string) (string, error)
	IsClean(dir string) (bool, error)
	Checkout(branch string) error
	Merge(branch string) error
	MergeInProgress() (bool, error)
	DetachedHEAD(dir string) (bool, error)
	MergeInProgressAt(dir string) (bool, error)
}

// TmuxClient is the tmux surface the orchestrator needs.
type TmuxClient interface {
	HasSession(name string) bool
	NewSession(name, dir string) error
	NewWindow(session, name, dir string) error
	KillWindow(session, name string) error
	Windows(session string) (map[string]string, error)
	SendCommand(session, window, command string) error
	Attach(session, window string) error

	PanesInWindow(session, window string) (map[string]string, error)
	NewWindowRunning(session, window, command string) (string, error)
	SplitWindow(session, window, command string) (string, error)
	KillPane(paneID string) error
	SetPaneTitle(paneID, title string) error
	SelectLayout(session, window, layout string) error
}

// StateStore persists the agent registry.
type StateStore interface {
	Load() (*state.State, error)
	Save(*state.State) error
}

// Notifier posts desktop notifications.
type Notifier interface {
	Notify(title, message string)
}

// FS is the filesystem surface the orchestrator needs for reconciliation.
type FS interface {
	DirExists(path string) bool
}

// Orchestrator wires the collaborators for all use-cases.
type Orchestrator struct {
	Git       GitClient
	Tmux      TmuxClient
	Store     StateStore
	FS        FS
	Cfg       *config.Config
	Providers provider.Registry
	UI        *ui.Printer
	Notifier  Notifier
	// Session is the resolved tmux session name for this repository.
	Session string
	// ExcludeWorktrees idempotently hides the worktree root from git status.
	ExcludeWorktrees func() error
	// WorktreePath maps an agent name to its absolute worktree path.
	WorktreePath func(name string) string
	// WatchPaneCommand builds the shell command a watch pane runs to mirror an
	// agent's window.
	WatchPaneCommand func(name string, interval time.Duration) string
	// LookPath reports whether an executable is resolvable on PATH. It is a
	// field so doctor's provider check stays testable without depending on the
	// real PATH of the test host.
	LookPath func(command string) error
}

var nameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// ValidateName enforces names usable as a branch, directory, and tmux window.
// Names must start with an alphanumeric, which also guarantees they can never
// collide with the reserved "_watch" dashboard window (see Watch).
func ValidateName(name string) error {
	if !nameRe.MatchString(name) {
		return fmt.Errorf("invalid agent name %q: use letters, digits, '.', '_' or '-' (no slashes, must start with a letter or digit)", name)
	}
	if strings.HasSuffix(name, ".lock") || strings.Contains(name, "..") {
		return fmt.Errorf("invalid agent name %q: not a valid git branch name", name)
	}
	if len(name) > 80 {
		return fmt.Errorf("invalid agent name %q: too long (max 80 characters)", name)
	}
	return nil
}

func (o *Orchestrator) agent(name string) (*state.State, state.Agent, error) {
	st, err := o.Store.Load()
	if err != nil {
		return nil, state.Agent{}, err
	}
	a, ok := st.Get(name)
	if !ok {
		return nil, state.Agent{}, fmt.Errorf("unknown agent %q (see `agents list`)", name)
	}
	return st, a, nil
}
