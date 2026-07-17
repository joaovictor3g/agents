package orchestrator

import (
	"fmt"
	"strings"

	"github.com/joaovictor3g/agents/internal/provider"
)

// planInstruction is prepended to the user's feature request. It pins the output
// to exactly the Markdown that ParsePlan consumes, so a successful plan can be
// fed straight to `agents spawn`. The format rules mirror ParsePlan's grammar
// and ValidateName's constraints on purpose.
const planInstruction = `You are decomposing a software feature request into a small team of specialized coding agents.

Output ONLY a Markdown plan and nothing else — no preamble, no explanation, no code fences.

Format, exactly:
- One "## <name>" heading per agent, where <name> is a short lowercase slug (letters, digits, and hyphens only, starting with a letter or digit) usable as a git branch — e.g. auth, payments, frontend, tests.
- Under each heading, one or more "- <task>" bullet lines describing that agent's work.
- Keep the team small and focused (typically 2-6 agents). No duplicate agent names.

Example:

## auth
- Add OAuth login and session handling

## payments
- Integrate Stripe billing and webhooks

## tests
- End-to-end coverage for the new flows

Feature request:
`

// Plan turns a high-level feature request into a Markdown plan: one "## <name>"
// heading per agent with "- <task>" bullets. It runs the resolved provider in
// headless mode, then validates the output round-trips through ParsePlan (the
// same parser `agents spawn` uses), retrying once with a corrective nudge if the
// model strays from the format. It only generates a plan — it never creates
// agents; that stays an explicit second step (`agents spawn`).
func (o *Orchestrator) Plan(request, providerName string) (string, error) {
	request = strings.TrimSpace(request)
	if request == "" {
		return "", fmt.Errorf("empty feature request")
	}
	if providerName == "" {
		providerName = o.Cfg.DefaultProvider
	}
	prov, err := o.Providers.Resolve(providerName)
	if err != nil {
		return "", err
	}

	md, err := o.generatePlan(prov, planInstruction+request)
	if err != nil {
		return "", err
	}

	// The plan is only useful if it parses. If the first attempt doesn't, retry
	// once telling the model what went wrong; models otherwise tend to wrap the
	// plan in prose or fences that ParsePlan would reject.
	if _, perr := ParsePlan(md); perr != nil {
		repair := fmt.Sprintf("%s%s\n\nYour previous answer was rejected (%v). Return ONLY the Markdown plan in the exact format described, with no extra text.", planInstruction, request, perr)
		md, err = o.generatePlan(prov, repair)
		if err != nil {
			return "", err
		}
		if _, perr := ParsePlan(md); perr != nil {
			return "", fmt.Errorf("provider %q did not return a valid plan (%w); its output was:\n\n%s", providerName, perr, md)
		}
	}
	return md, nil
}

// generatePlan runs the provider headlessly with the given prompt and returns
// its stdout with any surrounding code fence stripped.
func (o *Orchestrator) generatePlan(prov provider.Provider, prompt string) (string, error) {
	argv, err := prov.PlanCommand(prompt)
	if err != nil {
		return "", err
	}
	out, err := o.Run.Output(argv[0], argv[1:]...)
	if err != nil {
		return "", fmt.Errorf("running %s for planning: %w", prov.Name, err)
	}
	return stripFence(out), nil
}

// stripFence removes a single surrounding Markdown code fence (```
// or ```md ... ```) when the whole output is wrapped in one, so a fenced reply
// still parses. Unfenced output is returned unchanged.
func stripFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) < 2 {
		return s
	}
	lines = lines[1:] // drop the opening ``` (possibly ```md)
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) == "```" {
			lines = lines[:i]
			break
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
