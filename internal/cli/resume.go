package cli

import (
	"github.com/spf13/cobra"

	"github.com/joaovictor3g/agents/internal/ui"
)

func newResumeCmd(printer *ui.Printer) *cobra.Command {
	var attach bool

	cmd := &cobra.Command{
		Use:   "resume <name>",
		Short: "Rebuild a stopped agent's tmux window and restart its AI session",
		Long: `Bring a stopped agent back to life after a reboot or a killed tmux server.

The agent's branch, worktree, and registry entry survive across restarts;
only the tmux window and AI process are lost. resume rebuilds those in place,
reusing the same branch and worktree — it never creates a new branch.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := buildOrchestrator(printer)
			if err != nil {
				return err
			}
			return o.Resume(args[0], attach)
		},
	}

	cmd.Flags().BoolVarP(&attach, "attach", "a", false, "attach to the agent's window after resuming")
	return cmd
}
