package cli

import (
	"github.com/spf13/cobra"

	"github.com/joaovictor3g/agents/internal/orchestrator"
	"github.com/joaovictor3g/agents/internal/ui"
)

func newListCmd(printer *ui.Printer) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List agents and their status",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := buildOrchestrator(printer)
			if err != nil {
				return err
			}
			infos, err := o.List()
			if err != nil {
				return err
			}
			if len(infos) == 0 {
				printer.Info("No agents. Create one with %s.", printer.Bold("agents create <name>"))
				return nil
			}
			rows := make([][]string, len(infos))
			for i, info := range infos {
				rows[i] = []string{
					info.Agent.Name,
					paintStatus(printer, info.Status),
					info.Agent.Provider,
					info.Agent.Branch,
					info.Agent.Worktree,
				}
			}
			printer.Table([]string{"NAME", "STATUS", "PROVIDER", "BRANCH", "WORKTREE"}, rows)
			return nil
		},
	}
}

func paintStatus(printer *ui.Printer, s orchestrator.Status) string {
	switch s {
	case orchestrator.StatusRunning:
		return printer.Green(string(s))
	case orchestrator.StatusIdle:
		return printer.Yellow(string(s))
	default:
		return printer.Red(string(s))
	}
}
