package cli

import (
	"github.com/spf13/cobra"

	"github.com/joaovictor3g/agents/internal/orchestrator"
	"github.com/joaovictor3g/agents/internal/ui"
)

func newDeleteCmd(printer *ui.Printer) *cobra.Command {
	var opts orchestrator.DeleteOptions

	cmd := &cobra.Command{
		Use:     "delete <name>",
		Aliases: []string{"rm"},
		Short:   "Stop an agent and remove its worktree and tmux window",
		Long: `Stop an agent and remove its worktree and tmux window.

The branch is kept by default because it may hold the agent's only unmerged
work; pass --branch to delete it too. Deletion is idempotent: pieces already
removed by hand are skipped, so a half-broken agent can always be cleaned up.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := buildOrchestrator(printer)
			if err != nil {
				return err
			}
			return o.Delete(args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.DeleteBranch, "branch", false, "also delete the agent's branch")
	cmd.Flags().BoolVarP(&opts.Force, "force", "f", false, "discard uncommitted changes and unmerged branches")
	return cmd
}
