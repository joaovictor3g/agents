package cli

import (
	"github.com/spf13/cobra"

	"github.com/joaovictor3g/agents/internal/orchestrator"
	"github.com/joaovictor3g/agents/internal/ui"
)

func newCreateCmd(printer *ui.Printer) *cobra.Command {
	var opts orchestrator.CreateOptions

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create an agent: branch, worktree, tmux window, and AI session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := buildOrchestrator(printer)
			if err != nil {
				return err
			}
			opts.Name = args[0]

			name := opts.Provider
			if name == "" {
				name = o.Cfg.DefaultProvider
			}
			if p, err := o.Providers.Resolve(name); err == nil {
				if err := providerInstalled(p.Command); err != nil {
					printer.Warn("%v — the session will open but the command may fail", err)
				}
			}
			return o.Create(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Provider, "provider", "p", "", "AI provider to launch (default from config)")
	cmd.Flags().StringVarP(&opts.Template, "template", "t", "", "prompt template name or path to inject at startup")
	cmd.Flags().StringVar(&opts.Prompt, "prompt", "", "inline prompt to inject at startup")
	cmd.Flags().StringVarP(&opts.Base, "base", "b", "", "base ref for the new branch (default: the repo's default branch)")
	cmd.Flags().BoolVarP(&opts.Attach, "attach", "a", false, "attach to the agent's window after creation")
	cmd.MarkFlagsMutuallyExclusive("template", "prompt")
	return cmd
}
