// Package provider resolves which AI CLI to launch and builds its command
// line. Prompts are injected as arguments at launch, never typed into a
// running TUI, so startup timing can never race.
package provider

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/joaovictor3g/agents/internal/config"
)

// PromptPlaceholder is replaced with the prompt text inside promptArgs.
const PromptPlaceholder = "{{prompt}}"

// Provider is a resolved AI provider.
type Provider struct {
	Name       string
	Command    string
	Args       []string
	PromptArgs []string
	PlanArgs   []string
}

// Registry holds all configured providers.
type Registry struct {
	providers map[string]Provider
}

// NewRegistry builds a registry from configuration.
func NewRegistry(cfg *config.Config) Registry {
	providers := make(map[string]Provider, len(cfg.Providers))
	for name, pc := range cfg.Providers {
		providers[name] = Provider{
			Name:       name,
			Command:    pc.Command,
			Args:       pc.Args,
			PromptArgs: pc.PromptArgs,
			PlanArgs:   pc.PlanArgs,
		}
	}
	return Registry{providers: providers}
}

// Resolve returns the named provider or an error listing the known ones.
func (r Registry) Resolve(name string) (Provider, error) {
	p, ok := r.providers[name]
	if !ok || p.Command == "" {
		return Provider{}, fmt.Errorf("unknown provider %q (available: %s)", name, strings.Join(r.Names(), ", "))
	}
	return p, nil
}

// Names returns the configured provider names, sorted.
func (r Registry) Names() []string {
	names := make([]string, 0, len(r.providers))
	for name, p := range r.providers {
		if p.Command != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// CommandLine builds the shell command that launches the provider, injecting
// prompt via the provider's promptArgs when non-empty.
func (p Provider) CommandLine(prompt string) (string, error) {
	argv := append([]string{p.Command}, p.Args...)
	if prompt != "" {
		if len(p.PromptArgs) == 0 {
			return "", fmt.Errorf("provider %q does not support prompt injection (set promptArgs in its config)", p.Name)
		}
		for _, arg := range p.PromptArgs {
			argv = append(argv, strings.ReplaceAll(arg, PromptPlaceholder, prompt))
		}
	}
	quoted := make([]string, len(argv))
	for i, arg := range argv {
		quoted[i] = ShellQuote(arg)
	}
	return strings.Join(quoted, " "), nil
}

// PlanCommand builds the argv that runs the provider in headless planning mode,
// with the planning prompt substituted into planArgs. Unlike CommandLine the
// result is a raw argv (command first), because planning runs the provider
// directly and captures its stdout rather than typing into a tmux window, so no
// shell quoting is involved. A provider with no planArgs cannot plan.
func (p Provider) PlanCommand(prompt string) ([]string, error) {
	if len(p.PlanArgs) == 0 {
		return nil, fmt.Errorf("provider %q does not support planning (set planArgs in its config)", p.Name)
	}
	argv := append([]string{p.Command}, p.Args...)
	for _, arg := range p.PlanArgs {
		argv = append(argv, strings.ReplaceAll(arg, PromptPlaceholder, prompt))
	}
	return argv, nil
}

var safeArg = regexp.MustCompile(`^[a-zA-Z0-9@%+=:,./_-]+$`)

// ShellQuote quotes s for POSIX shells (and fish, which also treats a
// backslash-escaped quote outside single quotes as a literal quote).
func ShellQuote(s string) string {
	if safeArg.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
