package cli

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/joaovictor3g/agents/internal/execx"
	"github.com/joaovictor3g/agents/internal/orchestrator"
	"github.com/joaovictor3g/agents/internal/tmux"
	"github.com/joaovictor3g/agents/internal/ui"
)

func newWatchCmd(printer *ui.Printer) *cobra.Command {
	var interval time.Duration

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Open a read-only dashboard mirroring every agent in a tiled grid",
		Long: `Open a dashboard window that mirrors every agent side by side in a tiled
grid of read-only panes. The agents' real windows are untouched; to interact
with one, use "agents attach <name>". Re-run "agents watch" after creating or
deleting agents to re-sync the grid. Dismiss it by closing the window.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := buildOrchestrator(printer)
			if err != nil {
				return err
			}
			return o.Watch(orchestrator.WatchOptions{Interval: interval})
		},
	}

	cmd.Flags().DurationVarP(&interval, "interval", "i", 2*time.Second, "refresh interval for each mirror pane")
	return cmd
}

// newWatchPaneCmd is the hidden per-pane worker launched inside the dashboard
// window. It loops forever, capturing the target agent's window and redrawing
// this pane, so it is never invoked by users directly.
func newWatchPaneCmd(printer *ui.Printer) *cobra.Command {
	var interval time.Duration

	cmd := &cobra.Command{
		Use:    "__watch-pane <session> <name>",
		Hidden: true,
		Args:   cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			session, name := args[0], args[1]
			tm := tmux.New(execx.System{})
			for {
				width, height := tm.PaneSize()
				header := name
				body, err := tm.CapturePane(session, name)
				if err != nil {
					body = "agent gone — close this pane with Ctrl-b x"
				}
				printer.WatchFrame(header, body, width, height)
				time.Sleep(interval)
			}
		},
	}

	cmd.Flags().DurationVarP(&interval, "interval", "i", 2*time.Second, "refresh interval")
	return cmd
}
