package cli

import (
	"github.com/spf13/cobra"

	"github.com/joaovictor3g/agents/internal/ui"
)

func newSpawnCmd(printer *ui.Printer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spawn <plan.md>",
		Short: "Create a whole team of agents from a Markdown plan file",
		Long: `Read a Markdown plan and create every agent it declares, dispatching each
one's initial task.

The plan is one second-level heading per agent (the agent name), with bullet
lines as that agent's task(s):

  ## auth
  - OAuth integration

  ## payments
  - Stripe billing

Each agent is provisioned through the same path as ` + "`agents create`" + ` (branch,
worktree, tmux window, provider, injected prompt). Spawning continues past an
individual failure and reports how many agents were created and how many failed.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := buildOrchestrator(printer)
			if err != nil {
				return err
			}
			return o.SpawnFromPlan(args[0])
		},
	}
	return cmd
}
