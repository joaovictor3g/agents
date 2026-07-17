package cli

import (
	"github.com/spf13/cobra"

	"github.com/joaovictor3g/agents/internal/orchestrator"
	"github.com/joaovictor3g/agents/internal/ui"
)

func newStatusCmd(printer *ui.Printer) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show repository and agent health",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := buildOrchestrator(printer)
			if err != nil {
				return err
			}
			r, err := o.StatusReport()
			if err != nil {
				return err
			}

			printer.Info("%s  %s", printer.Bold("Repository:"), r.RepoRoot)
			if r.DetachedHEAD {
				printer.Warn("Detached HEAD in the main checkout")
			} else {
				printer.Info("%s      %s", printer.Bold("Branch:"), r.CurrentBranch)
			}
			if r.DefaultBranch != "" {
				printer.Info("%s     %s", printer.Bold("Default:"), r.DefaultBranch)
			}
			if r.MergeInProgress {
				printer.Warn("A merge is in progress in the main checkout — resolve it before merging agents")
			}
			session := r.Session
			if !r.SessionExists {
				session += printer.Dim(" (no tmux session)")
			}
			printer.Info("%s     %s", printer.Bold("Session:"), session)
			printer.Info("")

			if len(r.Agents) == 0 {
				printer.Info("No agents. Create one with %s.", printer.Bold("agents create <name>"))
				return nil
			}
			rows := make([][]string, len(r.Agents))
			for i, info := range r.Agents {
				hint := ""
				switch info.Status {
				case orchestrator.StatusDead, orchestrator.StatusBroken:
					hint = printer.Dim("agents delete " + info.Agent.Name)
				}
				rows[i] = []string{
					info.Agent.Name,
					paintStatus(printer, info.Status),
					info.Agent.Provider,
					info.Agent.Branch,
					hint,
				}
			}
			printer.Table([]string{"NAME", "STATUS", "PROVIDER", "BRANCH", "FIX"}, rows)
			return nil
		},
	}
}
