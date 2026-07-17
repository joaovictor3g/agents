package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/joaovictor3g/agents/internal/ui"
)

func newResumeCmd(printer *ui.Printer) *cobra.Command {
	var attach bool
	var all bool

	cmd := &cobra.Command{
		Use:   "resume [name]",
		Short: "Rebuild a stopped agent's tmux window and restart its AI session",
		Long: `Bring stopped agents back to life after a reboot or a killed tmux server.

The agent's branch, worktree, and registry entry survive across restarts;
only the tmux window and AI process are lost. resume rebuilds those in place,
reusing the same branch and worktree — it never creates a new branch.

Pass a name to resume one agent, or --all to resume every registered agent.
--all is idempotent: already-running agents are skipped, individual failures
are reported, and the run continues to the end rather than aborting.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// --all and a name are mutually exclusive; exactly one must be given.
			if all && len(args) > 0 {
				return fmt.Errorf("cannot combine --all with an agent name")
			}
			if !all && len(args) == 0 {
				return fmt.Errorf("provide an agent name or --all")
			}

			o, err := buildOrchestrator(printer)
			if err != nil {
				return err
			}
			if all {
				return o.ResumeAll()
			}
			return o.Resume(args[0], attach)
		},
	}

	cmd.Flags().BoolVarP(&attach, "attach", "a", false, "attach to the agent's window after resuming")
	cmd.Flags().BoolVar(&all, "all", false, "resume every registered agent")
	return cmd
}
