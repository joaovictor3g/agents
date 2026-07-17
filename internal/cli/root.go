// Package cli defines the cobra command tree. Commands are thin: they parse
// flags, build the orchestrator, and delegate; no policy lives here.
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/joaovictor3g/agents/internal/config"
	"github.com/joaovictor3g/agents/internal/execx"
	"github.com/joaovictor3g/agents/internal/git"
	"github.com/joaovictor3g/agents/internal/orchestrator"
	"github.com/joaovictor3g/agents/internal/platform"
	"github.com/joaovictor3g/agents/internal/provider"
	"github.com/joaovictor3g/agents/internal/state"
	"github.com/joaovictor3g/agents/internal/tmux"
	"github.com/joaovictor3g/agents/internal/ui"
	"github.com/joaovictor3g/agents/internal/worktree"
)

// Execute runs the CLI and returns the process exit code.
func Execute(version string) int {
	printer := ui.New()
	root := newRootCmd(version, printer)
	if err := root.Execute(); err != nil {
		printer.Error(err)
		return 1
	}
	return 0
}

func newRootCmd(version string, printer *ui.Printer) *cobra.Command {
	root := &cobra.Command{
		Use:   "agents",
		Short: "Orchestrate parallel AI coding agents in git worktrees and tmux windows",
		Long: `agents runs a team of terminal AI coding agents (Claude Code, Codex CLI,
Gemini CLI, ...) in parallel on one repository. Every agent owns exactly one
git branch, one git worktree, one tmux window, and one AI session, so agents
never interfere with each other.`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		newCreateCmd(printer),
		newListCmd(printer),
		newAttachCmd(printer),
		newDeleteCmd(printer),
		newMergeCmd(printer),
		newStatusCmd(printer),
	)
	return root
}

func buildOrchestrator(printer *ui.Printer) (*orchestrator.Orchestrator, error) {
	run := execx.System{}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	g, err := git.Discover(run, cwd)
	if err != nil {
		return nil, err
	}

	cfg, warnings, err := config.Load(g.Root())
	if err != nil {
		return nil, err
	}
	for _, w := range warnings {
		printer.Warn("%s", w)
	}

	tm := tmux.New(run)
	if !tm.Installed() {
		return nil, fmt.Errorf("tmux is not installed (brew install tmux)")
	}

	commonDir, err := g.CommonDir()
	if err != nil {
		return nil, err
	}

	session := cfg.Session
	if session == "" {
		session = filepath.Base(g.Root())
	}

	return &orchestrator.Orchestrator{
		Git:       g,
		Tmux:      tm,
		Store:     &state.Store{Path: filepath.Join(commonDir, "agents", "state.json")},
		FS:        osFS{},
		Cfg:       cfg,
		Providers: provider.NewRegistry(cfg),
		UI:        printer,
		Notifier:  platform.NewNotifier(run),
		Session:   session,
		ExcludeWorktrees: func() error {
			return worktree.EnsureExcluded(commonDir, g.Root(), cfg.WorktreesRoot)
		},
		WorktreePath: func(name string) string {
			return worktree.Path(g.Root(), cfg.WorktreesRoot, name)
		},
	}, nil
}

type osFS struct{}

func (osFS) DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// providerInstalled warns early when the provider binary is missing, so the
// failure surfaces before a window is created rather than inside it.
func providerInstalled(command string) error {
	if _, err := exec.LookPath(command); err != nil {
		return fmt.Errorf("provider command %q not found in PATH", command)
	}
	return nil
}
