package orchestrator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/joaovictor3g/agents/internal/state"
)

// problemsFor returns the messages of all problems reported for agent (or ""
// for repo-wide problems), so tests can assert on intent without pinning exact
// suggestion wording.
func problemsFor(d *Diagnosis, agent string) []Problem {
	var out []Problem
	for _, p := range d.Problems {
		if p.Agent == agent {
			out = append(out, p)
		}
	}
	return out
}

func TestDoctorHealthy(t *testing.T) {
	w := newWorld()
	w.git.branches["auth"] = true
	w.store.state.Add(state.Agent{Name: "auth", Provider: "claude", Branch: "auth", Worktree: "/repo/worktrees/auth"})
	w.tmux.windows["auth"] = "node"

	d, err := w.orch.Doctor()
	if err != nil {
		t.Fatal(err)
	}
	if !d.Healthy() {
		t.Fatalf("expected healthy, got %+v", d.Problems)
	}
}

func TestDoctorDetectsPerAgentProblems(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(w *world)
		wantSubstr string // substring expected in the agent's problem message
	}{
		{
			name: "missing worktree with live branch suggests resume",
			setup: func(w *world) {
				w.git.branches["auth"] = true
				w.fs.missing["/repo/worktrees/auth"] = true
			},
			wantSubstr: "worktree directory is missing",
		},
		{
			name: "missing worktree and branch suggests delete",
			setup: func(w *world) {
				w.fs.missing["/repo/worktrees/auth"] = true // branch not registered in git
			},
			wantSubstr: "both gone",
		},
		{
			name: "dead tmux window",
			setup: func(w *world) {
				w.git.branches["auth"] = true
				// no window registered
			},
			wantSubstr: "tmux window is missing",
		},
		{
			name: "missing provider executable",
			setup: func(w *world) {
				w.git.branches["auth"] = true
				w.tmux.windows["auth"] = "node"
				w.orch.LookPath = func(cmd string) error { return fmt.Errorf("%s not found", cmd) }
			},
			wantSubstr: "not on PATH",
		},
		{
			name: "detached HEAD in worktree",
			setup: func(w *world) {
				w.git.branches["auth"] = true
				w.tmux.windows["auth"] = "node"
				w.git.detachedDirs["/repo/worktrees/auth"] = true
			},
			wantSubstr: "detached HEAD",
		},
		{
			name: "merge in progress in worktree",
			setup: func(w *world) {
				w.git.branches["auth"] = true
				w.tmux.windows["auth"] = "node"
				w.git.mergingDirs["/repo/worktrees/auth"] = true
			},
			wantSubstr: "merge is in progress in the worktree",
		},
		{
			name: "dirty worktree",
			setup: func(w *world) {
				w.git.branches["auth"] = true
				w.tmux.windows["auth"] = "node"
				w.git.dirtyDirs["/repo/worktrees/auth"] = true
			},
			wantSubstr: "uncommitted changes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld()
			w.store.state.Add(state.Agent{Name: "auth", Provider: "claude", Branch: "auth", Worktree: "/repo/worktrees/auth"})
			tt.setup(w)

			d, err := w.orch.Doctor()
			if err != nil {
				t.Fatal(err)
			}
			if d.Healthy() {
				t.Fatalf("expected a problem, got none")
			}
			problems := problemsFor(d, "auth")
			found := false
			for _, p := range problems {
				if strings.Contains(p.Message, tt.wantSubstr) {
					found = true
				}
				if p.Suggestion == "" {
					t.Errorf("problem %q has no suggestion", p.Message)
				}
			}
			if !found {
				t.Fatalf("no problem contained %q; got %+v", tt.wantSubstr, problems)
			}
		})
	}
}

func TestDoctorMissingWorktreeSkipsDeeperChecks(t *testing.T) {
	// A gone worktree must not also emit detached/merge/dirty noise: those
	// checks are impossible without a worktree, so exactly one problem is right.
	w := newWorld()
	w.git.branches["auth"] = true
	w.fs.missing["/repo/worktrees/auth"] = true
	w.git.detachedDirs["/repo/worktrees/auth"] = true // would fire if reached
	w.store.state.Add(state.Agent{Name: "auth", Provider: "claude", Branch: "auth", Worktree: "/repo/worktrees/auth"})

	d, err := w.orch.Doctor()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(problemsFor(d, "auth")); got != 1 {
		t.Fatalf("want exactly 1 problem for a missing worktree, got %d: %+v", got, d.Problems)
	}
}

func TestDoctorDetectsRepoWideProblems(t *testing.T) {
	w := newWorld()
	w.git.currentBranch = "" // detached HEAD in the main checkout
	w.git.mergeInFlight = true

	d, err := w.orch.Doctor()
	if err != nil {
		t.Fatal(err)
	}
	repo := problemsFor(d, "")
	if len(repo) != 2 {
		t.Fatalf("want 2 repo-wide problems, got %d: %+v", len(repo), repo)
	}
}

func TestDoctorReportsMultipleProblemsPerAgent(t *testing.T) {
	w := newWorld()
	w.git.branches["auth"] = true
	w.store.state.Add(state.Agent{Name: "auth", Provider: "claude", Branch: "auth", Worktree: "/repo/worktrees/auth"})
	// Window dead AND worktree dirty: both should surface independently.
	w.git.dirtyDirs["/repo/worktrees/auth"] = true

	d, err := w.orch.Doctor()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(problemsFor(d, "auth")); got != 2 {
		t.Fatalf("want 2 problems (dead window + dirty), got %d: %+v", got, d.Problems)
	}
}
