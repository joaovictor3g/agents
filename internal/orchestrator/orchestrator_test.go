package orchestrator

import (
	"strings"
	"testing"

	"github.com/joaovictor3g/agents/internal/state"
)

func TestValidateName(t *testing.T) {
	valid := []string{"auth", "tests-2", "review_ui", "a", "Auth.v2"}
	for _, name := range valid {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", name, err)
		}
	}
	invalid := []string{"", "feat/auth", "-auth", ".auth", "a b", "auth.lock", "a..b", strings.Repeat("x", 81)}
	for _, name := range invalid {
		if err := ValidateName(name); err == nil {
			t.Errorf("ValidateName(%q) = nil, want error", name)
		}
	}
}

func TestCreateHappyPath(t *testing.T) {
	w := newWorld()
	err := w.orch.Create(CreateOptions{Name: "auth"})
	if err != nil {
		t.Fatal(err)
	}

	if !w.git.branches["auth"] {
		t.Error("branch auth not created")
	}
	if _, ok := w.tmux.windows["auth"]; !ok {
		t.Error("tmux window not created")
	}
	if got := w.tmux.sent["auth"]; got != "claude" {
		t.Errorf("sent command = %q, want claude", got)
	}
	a, ok := w.store.state.Get("auth")
	if !ok {
		t.Fatal("agent not registered")
	}
	if a.Provider != "claude" || a.Branch != "auth" || a.Worktree != "/repo/worktrees/auth" {
		t.Errorf("registry entry wrong: %+v", a)
	}
	if w.tmux.attached != "" {
		t.Error("create must not attach without --attach")
	}
}

func TestCreateWithPromptInjectsArgv(t *testing.T) {
	w := newWorld()
	if err := w.orch.Create(CreateOptions{Name: "review", Prompt: "review this PR"}); err != nil {
		t.Fatal(err)
	}
	if got := w.tmux.sent["review"]; got != `claude 'review this PR'` {
		t.Errorf("sent = %q", got)
	}
}

func TestCreateDuplicateFails(t *testing.T) {
	w := newWorld()
	w.store.state.Add(state.Agent{Name: "auth"})
	err := w.orch.Create(CreateOptions{Name: "auth"})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err = %v", err)
	}
}

func TestCreateAdoptsExistingBranch(t *testing.T) {
	w := newWorld()
	w.git.branches["auth"] = true
	if err := w.orch.Create(CreateOptions{Name: "auth"}); err != nil {
		t.Fatal(err)
	}
	for _, entry := range w.git.log {
		if strings.HasPrefix(entry, "add-worktree-new") {
			t.Fatalf("should adopt existing branch, not create: %v", w.git.log)
		}
	}
}

func TestCreateRefusesBranchCheckedOutElsewhere(t *testing.T) {
	w := newWorld()
	w.git.branches["auth"] = true
	w.git.checkedOutAt["auth"] = "/repo/other"
	err := w.orch.Create(CreateOptions{Name: "auth"})
	if err == nil || !strings.Contains(err.Error(), "checked out") {
		t.Fatalf("err = %v", err)
	}
}

func TestCreateUsesBaseFlag(t *testing.T) {
	w := newWorld()
	if err := w.orch.Create(CreateOptions{Name: "auth-tests", Base: "auth"}); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range w.git.log {
		if entry == "add-worktree-new /repo/worktrees/auth-tests auth-tests from auth" {
			found = true
		}
	}
	if !found {
		t.Fatalf("base not honored: %v", w.git.log)
	}
}

func TestCreateRejectsOptionLikeBase(t *testing.T) {
	w := newWorld()
	err := w.orch.Create(CreateOptions{Name: "auth", Base: "--upload-pack=touch /tmp/pwned"})
	if err == nil || !strings.Contains(err.Error(), "cannot start with '-'") {
		t.Fatalf("err = %v, want rejection of option-like base", err)
	}
	if len(w.git.log) != 0 {
		t.Errorf("nothing should be touched when base is rejected: %v", w.git.log)
	}
}

func TestCreateAttachFlag(t *testing.T) {
	w := newWorld()
	if err := w.orch.Create(CreateOptions{Name: "auth", Attach: true}); err != nil {
		t.Fatal(err)
	}
	if w.tmux.attached != "repo:auth" {
		t.Errorf("attached = %q", w.tmux.attached)
	}
}

func TestListStatuses(t *testing.T) {
	w := newWorld()
	w.store.state.Add(state.Agent{Name: "auth", Worktree: "/repo/worktrees/auth"})
	w.store.state.Add(state.Agent{Name: "tests", Worktree: "/repo/worktrees/tests"})
	w.store.state.Add(state.Agent{Name: "dead1", Worktree: "/repo/worktrees/dead1"})
	w.store.state.Add(state.Agent{Name: "broken1", Worktree: "/repo/worktrees/broken1"})

	w.tmux.windows["auth"] = "node"
	w.tmux.windows["tests"] = "zsh"
	w.fs.missing["/repo/worktrees/broken1"] = true

	infos, err := w.orch.List()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Status{
		"auth":    StatusRunning,
		"tests":   StatusIdle,
		"dead1":   StatusDead,
		"broken1": StatusBroken,
	}
	for _, info := range infos {
		if info.Status != want[info.Agent.Name] {
			t.Errorf("%s: status = %s, want %s", info.Agent.Name, info.Status, want[info.Agent.Name])
		}
	}
}

func TestDeleteKeepsBranchByDefault(t *testing.T) {
	w := newWorld()
	w.git.branches["auth"] = true
	w.store.state.Add(state.Agent{Name: "auth", Branch: "auth", Worktree: "/repo/worktrees/auth"})
	w.tmux.windows["auth"] = "node"

	if err := w.orch.Delete("auth", DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	if !w.git.branches["auth"] {
		t.Error("branch must be kept by default")
	}
	if _, ok := w.store.state.Get("auth"); ok {
		t.Error("registry entry not removed")
	}
	if _, ok := w.tmux.windows["auth"]; ok {
		t.Error("window not killed")
	}
}

func TestDeleteBranchFlag(t *testing.T) {
	w := newWorld()
	w.git.branches["auth"] = true
	w.store.state.Add(state.Agent{Name: "auth", Branch: "auth", Worktree: "/repo/worktrees/auth"})

	if err := w.orch.Delete("auth", DeleteOptions{DeleteBranch: true}); err != nil {
		t.Fatal(err)
	}
	if w.git.branches["auth"] {
		t.Error("branch should be deleted with --branch")
	}
}

func TestDeleteIsIdempotentOverMissingPieces(t *testing.T) {
	w := newWorld()
	w.store.state.Add(state.Agent{Name: "auth", Branch: "auth", Worktree: "/repo/worktrees/auth"})
	w.fs.missing["/repo/worktrees/auth"] = true

	if err := w.orch.Delete("auth", DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, ok := w.store.state.Get("auth"); ok {
		t.Error("registry entry not removed")
	}
}

func TestDeleteDirtyWorktreeNeedsForce(t *testing.T) {
	w := newWorld()
	w.store.state.Add(state.Agent{Name: "auth", Branch: "auth", Worktree: "/repo/worktrees/auth"})
	w.git.dirtyDirs["/repo/worktrees/auth"] = true

	err := w.orch.Delete("auth", DeleteOptions{})
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("err = %v", err)
	}
	if _, ok := w.store.state.Get("auth"); !ok {
		t.Error("registry entry must survive a failed delete")
	}

	if err := w.orch.Delete("auth", DeleteOptions{Force: true}); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteUnknownAgent(t *testing.T) {
	w := newWorld()
	err := w.orch.Delete("ghost", DeleteOptions{})
	if err == nil || !strings.Contains(err.Error(), "unknown agent") {
		t.Fatalf("err = %v", err)
	}
}

func TestMergeHappyPathTearsDown(t *testing.T) {
	w := newWorld()
	w.git.branches["auth"] = true
	w.git.currentBranch = "other"
	w.store.state.Add(state.Agent{Name: "auth", Branch: "auth", Worktree: "/repo/worktrees/auth"})
	w.tmux.windows["auth"] = "node"

	if err := w.orch.Merge("auth", MergeOptions{}); err != nil {
		t.Fatal(err)
	}
	if w.git.currentBranch != "main" {
		t.Errorf("should have checked out main, on %s", w.git.currentBranch)
	}
	if w.git.branches["auth"] {
		t.Error("merged branch should be deleted by default")
	}
	if _, ok := w.tmux.windows["auth"]; ok {
		t.Error("window not killed")
	}
	if _, ok := w.store.state.Get("auth"); ok {
		t.Error("registry entry not removed")
	}
}

func TestMergeKeepBranch(t *testing.T) {
	w := newWorld()
	w.git.branches["auth"] = true
	w.store.state.Add(state.Agent{Name: "auth", Branch: "auth", Worktree: "/repo/worktrees/auth"})

	if err := w.orch.Merge("auth", MergeOptions{KeepBranch: true}); err != nil {
		t.Fatal(err)
	}
	if !w.git.branches["auth"] {
		t.Error("--keep-branch must keep the branch")
	}
}

func TestMergeAbortsOnDirtyMainCheckout(t *testing.T) {
	w := newWorld()
	w.git.branches["auth"] = true
	w.git.dirtyDirs["/repo"] = true
	w.store.state.Add(state.Agent{Name: "auth", Branch: "auth", Worktree: "/repo/worktrees/auth"})

	err := w.orch.Merge("auth", MergeOptions{})
	if err == nil || !strings.Contains(err.Error(), "main checkout") {
		t.Fatalf("err = %v", err)
	}
	if len(w.git.log) != 0 {
		t.Errorf("nothing should be touched, git log: %v", w.git.log)
	}
}

func TestMergeAbortsOnDirtyAgentWorktree(t *testing.T) {
	w := newWorld()
	w.git.branches["auth"] = true
	w.git.dirtyDirs["/repo/worktrees/auth"] = true
	w.store.state.Add(state.Agent{Name: "auth", Branch: "auth", Worktree: "/repo/worktrees/auth"})

	err := w.orch.Merge("auth", MergeOptions{})
	if err == nil || !strings.Contains(err.Error(), "uncommitted changes") {
		t.Fatalf("err = %v", err)
	}
}

func TestMergeConflictLeavesEverythingIntact(t *testing.T) {
	w := newWorld()
	w.git.branches["auth"] = true
	w.git.mergeConflict = true
	w.store.state.Add(state.Agent{Name: "auth", Branch: "auth", Worktree: "/repo/worktrees/auth"})
	w.tmux.windows["auth"] = "node"

	err := w.orch.Merge("auth", MergeOptions{})
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("err = %v", err)
	}
	if !w.git.branches["auth"] {
		t.Error("branch must survive a conflict")
	}
	if _, ok := w.tmux.windows["auth"]; !ok {
		t.Error("window must survive a conflict")
	}
	if _, ok := w.store.state.Get("auth"); !ok {
		t.Error("registry entry must survive a conflict")
	}
}

func TestMergeRefusesWhenMergeAlreadyInProgress(t *testing.T) {
	w := newWorld()
	w.git.branches["auth"] = true
	w.git.mergeInFlight = true
	w.store.state.Add(state.Agent{Name: "auth", Branch: "auth", Worktree: "/repo/worktrees/auth"})

	err := w.orch.Merge("auth", MergeOptions{})
	if err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("err = %v", err)
	}
}

func TestStatusReport(t *testing.T) {
	w := newWorld()
	w.git.currentBranch = ""
	w.store.state.Add(state.Agent{Name: "auth", Worktree: "/repo/worktrees/auth"})

	r, err := w.orch.StatusReport()
	if err != nil {
		t.Fatal(err)
	}
	if !r.DetachedHEAD {
		t.Error("empty current branch should report detached HEAD")
	}
	if len(r.Agents) != 1 {
		t.Errorf("agents = %d", len(r.Agents))
	}
}
