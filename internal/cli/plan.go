package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joaovictor3g/agents/internal/ui"
)

func newPlanCmd(printer *ui.Printer) *cobra.Command {
	var providerName string
	var out string

	cmd := &cobra.Command{
		Use:   "plan <request>",
		Short: "Decompose a feature request into a Markdown team plan",
		Long: `Ask a provider to decompose a high-level feature request into a small team of
specialized agents, and emit the result as a Markdown plan.

The plan is one second-level heading per agent (the agent name), with bullet
lines as that agent's task(s) — the exact format ` + "`agents spawn`" + ` reads:

  ## auth
  - OAuth login and session handling

  ## payments
  - Stripe billing and webhooks

This command only generates a plan; it never creates agents. Review (and edit)
the plan, then run ` + "`agents spawn <plan.md>`" + ` to create the team.

With no --out the plan is written to stdout, so it can be redirected or piped:

  agents plan "Implement Stripe subscriptions" > plan.md`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := buildOrchestrator(printer)
			if err != nil {
				return err
			}
			plan, err := o.Plan(strings.Join(args, " "), providerName)
			if err != nil {
				return err
			}
			if out == "" {
				fmt.Fprintln(cmd.OutOrStdout(), plan)
				return nil
			}
			if err := os.WriteFile(out, []byte(plan+"\n"), 0o644); err != nil {
				return fmt.Errorf("writing plan: %w", err)
			}
			printer.Success("Wrote plan to %s", printer.Bold(out))
			printer.Info("Review it, then run %s to create the team.", printer.Bold("agents spawn "+out))
			return nil
		},
	}
	cmd.Flags().StringVarP(&providerName, "provider", "p", "", "provider to plan with (default: configured default)")
	cmd.Flags().StringVarP(&out, "out", "o", "", "write the plan to a file instead of stdout")
	return cmd
}
