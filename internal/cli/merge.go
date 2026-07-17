package cli

import (
	"github.com/spf13/cobra"

	"github.com/victordias/agents/internal/orchestrator"
	"github.com/victordias/agents/internal/ui"
)

func newMergeCmd(printer *ui.Printer) *cobra.Command {
	var opts orchestrator.MergeOptions

	cmd := &cobra.Command{
		Use:   "merge <name>",
		Short: "Merge an agent's branch into the default branch and tear the agent down",
		Long: `Merge an agent's branch into the repository's default branch, then remove
the agent's worktree, tmux window, and (by default) its merged branch.

The merge refuses to start if the main checkout or the agent's worktree has
uncommitted changes. On conflicts the merge is left in progress for manual
resolution and nothing is torn down.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := buildOrchestrator(printer)
			if err != nil {
				return err
			}
			return o.Merge(args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.KeepBranch, "keep-branch", false, "keep the branch after merging")
	cmd.Flags().BoolVarP(&opts.Force, "force", "f", false, "merge even if the agent worktree has uncommitted changes")
	return cmd
}
