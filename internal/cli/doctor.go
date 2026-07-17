package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/joaovictor3g/agents/internal/ui"
)

func newDoctorCmd(printer *ui.Printer) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose common problems and suggest fixes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := buildOrchestrator(printer)
			if err != nil {
				return err
			}
			d, err := o.Doctor()
			if err != nil {
				return err
			}
			if d.Healthy() {
				printer.Success("No problems found. All agents are healthy.")
				return nil
			}

			for _, p := range d.Problems {
				subject := p.Message
				if p.Agent != "" {
					subject = printer.Bold(p.Agent) + ": " + p.Message
				}
				printer.Warn("%s", subject)
				printer.Info("  %s %s", printer.Dim("→"), p.Suggestion)
			}
			printer.Info("")
			// Return an error so the process exits non-zero for scripts; the
			// per-problem detail is already printed above.
			return fmt.Errorf("found %d problem(s)", len(d.Problems))
		},
	}
}
