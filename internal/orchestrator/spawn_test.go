package orchestrator

import (
	"os"
	"strings"
	"testing"

	"github.com/joaovictor3g/agents/internal/state"
)

func TestParsePlan(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []PlanEntry
		wantErr string
	}{
		{
			name: "valid multi-agent plan",
			input: `# My plan

## auth
- OAuth integration

## payments
- Stripe billing
- Refunds

## tests
- Playwright coverage
`,
			want: []PlanEntry{
				{Name: "auth", Prompt: "OAuth integration"},
				{Name: "payments", Prompt: "Stripe billing\nRefunds"},
				{Name: "tests", Prompt: "Playwright coverage"},
			},
		},
		{
			name:  "star and plus bullets, prose ignored",
			input: "## auth\nsome prose that is not a bullet\n* first\n+ second\n",
			want:  []PlanEntry{{Name: "auth", Prompt: "first\nsecond"}},
		},
		{
			name:    "no headings",
			input:   "just some text\n- a bullet\n",
			wantErr: "no agents found",
		},
		{
			name:    "duplicate agent name",
			input:   "## auth\n- one\n## auth\n- two\n",
			wantErr: "duplicate agent",
		},
		{
			name:    "empty tasks",
			input:   "## auth\n\n## payments\n- Stripe\n",
			wantErr: "has no tasks",
		},
		{
			name:    "invalid agent name",
			input:   "## feat/auth\n- work\n",
			wantErr: "invalid agent name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePlan(tt.input)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d entries, want %d: %+v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("entry %d = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSpawnFromPlanAllSucceed(t *testing.T) {
	w := newWorld()
	path := writePlan(t, "## auth\n- OAuth\n\n## payments\n- Stripe\n")

	if err := w.orch.SpawnFromPlan(path); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	for _, name := range []string{"auth", "payments"} {
		if !w.git.branches[name] {
			t.Errorf("branch %s not created", name)
		}
		if _, ok := w.tmux.windows[name]; !ok {
			t.Errorf("tmux window %s not created", name)
		}
		if _, ok := w.store.state.Get(name); !ok {
			t.Errorf("agent %s not registered", name)
		}
	}
	if got := w.tmux.sent["auth"]; got != "claude OAuth" {
		t.Errorf("auth command = %q", got)
	}
}

func TestSpawnFromPlanPartialFailure(t *testing.T) {
	w := newWorld()
	// "auth" already exists, so its Create fails; "payments" should still spawn.
	w.store.state.Add(state.Agent{Name: "auth"})
	path := writePlan(t, "## auth\n- OAuth\n\n## payments\n- Stripe\n")

	err := w.orch.SpawnFromPlan(path)
	if err == nil || !strings.Contains(err.Error(), "1 of 2 agents failed") {
		t.Fatalf("err = %v, want summarizing failure", err)
	}
	if _, ok := w.tmux.windows["payments"]; !ok {
		t.Error("payments should spawn despite auth failing")
	}
	if !strings.Contains(w.out.String(), "1 created, 1 failed") {
		t.Errorf("missing summary in output:\n%s", w.out.String())
	}
}

func TestSpawnFromPlanMissingFile(t *testing.T) {
	w := newWorld()
	if err := w.orch.SpawnFromPlan("/no/such/plan.md"); err == nil {
		t.Fatal("expected error for missing plan file")
	}
}

func TestSpawnFromPlanMalformedFile(t *testing.T) {
	w := newWorld()
	path := writePlan(t, "no headings here\n")
	if err := w.orch.SpawnFromPlan(path); err == nil || !strings.Contains(err.Error(), "no agents found") {
		t.Fatalf("err = %v, want parse error", err)
	}
}

func writePlan(t *testing.T, content string) string {
	t.Helper()
	path := t.TempDir() + "/plan.md"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
