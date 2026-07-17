package orchestrator

// Status is an agent's reconciled liveness, derived from tmux and the
// filesystem at read time; the registry never stores it.
type Status string

const (
	// StatusRunning means the provider process is active in the window.
	StatusRunning Status = "running"
	// StatusIdle means the window is alive but the provider exited to a shell.
	StatusIdle Status = "idle"
	// StatusDead means the tmux window is gone.
	StatusDead Status = "dead"
	// StatusBroken means the worktree directory is missing.
	StatusBroken Status = "broken"
)

var shells = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "fish": true,
	"dash": true, "ksh": true, "tcsh": true, "csh": true, "nu": true,
}

func statusOf(worktreeExists, windowExists bool, paneCommand string) Status {
	switch {
	case !worktreeExists:
		return StatusBroken
	case !windowExists:
		return StatusDead
	case shells[paneCommand]:
		return StatusIdle
	default:
		return StatusRunning
	}
}
