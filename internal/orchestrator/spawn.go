package orchestrator

import (
	"fmt"
	"os"
	"strings"
)

// PlanEntry is one agent parsed from a plan file: the agent name and the task
// prompt injected at startup.
type PlanEntry struct {
	Name   string
	Prompt string
}

// SpawnFromPlan reads a Markdown plan file and creates every agent it declares,
// dispatching each one's initial task. It reuses the full Create path per agent
// (branch + worktree + tmux window + provider + injected prompt), so nothing
// about provisioning is duplicated here.
//
// Like the bulk-resume flow, spawn continues past individual failures rather
// than aborting the whole batch: agents created before a failure keep running,
// each remaining agent is still attempted, and a summary is printed at the end.
// A non-nil error is returned when any agent failed so the process exits non-zero.
func (o *Orchestrator) SpawnFromPlan(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	entries, err := ParsePlan(string(data))
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	var failed int
	for _, e := range entries {
		// Default provider/template per agent: Create resolves the default
		// provider when Provider is empty. A future plan format could carry
		// per-agent provider/template annotations that would populate these.
		if err := o.Create(CreateOptions{Name: e.Name, Prompt: e.Prompt}); err != nil {
			o.UI.Warn("agent %s: %v", o.UI.Bold(e.Name), err)
			failed++
			continue
		}
	}

	created := len(entries) - failed
	o.UI.Info("")
	if failed > 0 {
		o.UI.Warn("%d created, %d failed", created, failed)
		return fmt.Errorf("%d of %d agents failed to spawn", failed, len(entries))
	}
	o.UI.Success("Spawned %d agents from %s", created, path)
	return nil
}

// ParsePlan turns a Markdown plan into an ordered list of agents. The format is
// one second-level heading per agent (the agent name), with bullet lines as
// that agent's task(s):
//
//	## auth
//	- OAuth integration
//
//	## payments
//	- Stripe billing
//
// Bullets under a heading are joined into a single task prompt. A level-1 title
// and any other prose are ignored. The parser rejects a plan with no agents, a
// duplicate agent name, an invalid agent name, or an agent with no tasks.
func ParsePlan(content string) ([]PlanEntry, error) {
	var entries []PlanEntry
	seen := map[string]bool{}

	// index into entries for the heading currently being filled, or -1 before
	// the first heading (bullets before any heading are ignored).
	cur := -1

	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)

		// A level-2 heading opens a new agent. "### " and deeper do not match
		// because the third rune is not a space.
		if strings.HasPrefix(line, "## ") {
			name := strings.TrimSpace(strings.TrimPrefix(line, "##"))
			if err := ValidateName(name); err != nil {
				return nil, err
			}
			if seen[name] {
				return nil, fmt.Errorf("duplicate agent %q", name)
			}
			seen[name] = true
			entries = append(entries, PlanEntry{Name: name})
			cur = len(entries) - 1
			continue
		}

		if cur < 0 {
			continue
		}
		if task, ok := bulletText(line); ok && task != "" {
			if entries[cur].Prompt == "" {
				entries[cur].Prompt = task
			} else {
				entries[cur].Prompt += "\n" + task
			}
		}
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no agents found: expected a '## <name>' heading per agent")
	}
	for _, e := range entries {
		if e.Prompt == "" {
			return nil, fmt.Errorf("agent %q has no tasks: add at least one '- <task>' bullet", e.Name)
		}
	}
	return entries, nil
}

// bulletText extracts the task from a Markdown list item ("- task", "* task",
// "+ task"), returning ok=false for any non-bullet line.
func bulletText(line string) (string, bool) {
	for _, marker := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(line, marker) {
			return strings.TrimSpace(line[len(marker):]), true
		}
	}
	return "", false
}
