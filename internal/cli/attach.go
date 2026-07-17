package cli

import (
	"github.com/spf13/cobra"

	"github.com/joaovictor3g/agents/internal/ui"
)

func newAttachCmd(printer *ui.Printer) *cobra.Command {
	return &cobra.Command{
		Use:   "attach <name>",
		Short: "Switch to an agent's tmux window",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := buildOrchestrator(printer)
			if err != nil {
				return err
			}
			return o.Attach(args[0])
		},
	}
}
