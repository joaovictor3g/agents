package orchestrator

import (
	"fmt"
	"strings"
	"testing"
)

const validPlan = "## auth\n- OAuth login\n\n## payments\n- Stripe billing\n"

func TestPlanReturnsValidMarkdown(t *testing.T) {
	w := newWorld()
	w.runner.outputs = []string{validPlan}

	got, err := w.orch.Plan("Implement Stripe subscriptions", "")
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if _, perr := ParsePlan(got); perr != nil {
		t.Fatalf("returned plan does not parse: %v", perr)
	}
	if len(w.runner.calls) != 1 {
		t.Fatalf("expected 1 provider call, got %d", len(w.runner.calls))
	}

	// Default provider (claude) must be invoked headlessly with the request
	// embedded in the planning prompt.
	call := w.runner.calls[0]
	if call[0] != "claude" || call[1] != "-p" {
		t.Errorf("expected headless `claude -p ...`, got %v", call[:2])
	}
	prompt := call[len(call)-1]
	if !strings.Contains(prompt, "Implement Stripe subscriptions") {
		t.Errorf("planning prompt missing the feature request: %q", prompt)
	}
}

func TestPlanStripsCodeFence(t *testing.T) {
	w := newWorld()
	w.runner.outputs = []string{"```md\n" + validPlan + "```"}

	got, err := w.orch.Plan("anything", "")
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if strings.Contains(got, "```") {
		t.Errorf("fence not stripped: %q", got)
	}
	if _, perr := ParsePlan(got); perr != nil {
		t.Errorf("fenced plan did not parse after stripping: %v", perr)
	}
}

func TestPlanRepairsOnInvalidFirstAttempt(t *testing.T) {
	w := newWorld()
	// First reply is prose with no headings (ParsePlan rejects it); the retry
	// returns a valid plan.
	w.runner.outputs = []string{"Sure! Here is how I'd split the work.", validPlan}

	got, err := w.orch.Plan("build it", "")
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if _, perr := ParsePlan(got); perr != nil {
		t.Fatalf("returned plan does not parse: %v", perr)
	}
	if len(w.runner.calls) != 2 {
		t.Fatalf("expected a repair retry (2 calls), got %d", len(w.runner.calls))
	}
	if repair := w.runner.calls[1][len(w.runner.calls[1])-1]; !strings.Contains(repair, "rejected") {
		t.Errorf("repair prompt should tell the model it was rejected: %q", repair)
	}
}

func TestPlanFailsWhenModelNeverParses(t *testing.T) {
	w := newWorld()
	w.runner.outputs = []string{"nope", "still nope"}

	_, err := w.orch.Plan("build it", "")
	if err == nil {
		t.Fatal("expected an error when the model never returns a valid plan")
	}
	// The error should name the provider and surface the raw output for the user.
	if !strings.Contains(err.Error(), "claude") || !strings.Contains(err.Error(), "still nope") {
		t.Errorf("error should name the provider and include its output: %v", err)
	}
}

func TestPlanRejectsEmptyRequest(t *testing.T) {
	w := newWorld()
	if _, err := w.orch.Plan("   ", ""); err == nil {
		t.Fatal("expected an error for an empty request")
	}
	if len(w.runner.calls) != 0 {
		t.Errorf("no provider should run for an empty request, got %d calls", len(w.runner.calls))
	}
}

func TestPlanSurfacesProviderError(t *testing.T) {
	w := newWorld()
	w.runner.outputs = []string{""}
	w.runner.errs = []error{fmt.Errorf("claude: command not found")}

	if _, err := w.orch.Plan("build it", ""); err == nil {
		t.Fatal("expected the provider execution error to propagate")
	}
}
