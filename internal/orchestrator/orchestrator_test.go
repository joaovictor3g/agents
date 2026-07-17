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

func TestResumeRebuildsDeadWindow(t *testing.T) {
	w := newWorld()
	w.git.branches["auth"] = true
	w.store.state.Add(state.Agent{Name: "auth", Provider: "claude", Branch: "auth", Worktree: "/repo/worktrees/auth"})
	// Reboot state: worktree on disk, but no tmux session or window.

	if err := w.orch.Resume("auth", false); err != nil {
		t.Fatal(err)
	}
	if !w.tmux.sessions["repo"] {
		t.Error("session should be recreated")
	}
	if _, ok := w.tmux.windows["auth"]; !ok {
		t.Error("window should be recreated")
	}
	if got := w.tmux.sent["auth"]; got != "claude" {
		t.Errorf("provider command = %q, want %q", got, "claude")
	}
	// No new branch: the existing branch is reused, git untouched.
	if len(w.git.log) != 0 {
		t.Errorf("git should not be touched when the worktree exists: %v", w.git.log)
	}
}

func TestResumeAttachesWhenAlreadyRunning(t *testing.T) {
	w := newWorld()
	w.store.state.Add(state.Agent{Name: "auth", Provider: "claude", Branch: "auth", Worktree: "/repo/worktrees/auth"})
	w.tmux.sessions["repo"] = true
	w.tmux.windows["auth"] = "claude"

	if err := w.orch.Resume("auth", true); err != nil {
		t.Fatal(err)
	}
	if w.tmux.attached != "repo:auth" {
		t.Errorf("attached = %q, want repo:auth", w.tmux.attached)
	}
	if _, sent := w.tmux.sent["auth"]; sent {
		t.Error("a running agent must not be relaunched")
	}
}

func TestResumeRestoresMissingWorktree(t *testing.T) {
	w := newWorld()
	w.git.branches["auth"] = true
	w.fs.missing["/repo/worktrees/auth"] = true // worktree dir gone, branch remains
	w.store.state.Add(state.Agent{Name: "auth", Provider: "claude", Branch: "auth", Worktree: "/repo/worktrees/auth"})

	if err := w.orch.Resume("auth", false); err != nil {
		t.Fatal(err)
	}
	if w.git.checkedOutAt["auth"] != "/repo/worktrees/auth" {
		t.Error("worktree should be re-added for the existing branch")
	}
	if _, ok := w.tmux.windows["auth"]; !ok {
		t.Error("window should be recreated after restoring the worktree")
	}
}

func TestResumeFailsWhenBranchGone(t *testing.T) {
	w := newWorld()
	w.fs.missing["/repo/worktrees/auth"] = true // worktree gone
	// branch "auth" does not exist in git
	w.store.state.Add(state.Agent{Name: "auth", Provider: "claude", Branch: "auth", Worktree: "/repo/worktrees/auth"})

	err := w.orch.Resume("auth", false)
	if err == nil || !strings.Contains(err.Error(), "no longer exists") {
		t.Fatalf("err = %v, want branch-gone error", err)
	}
}

func TestResumeUnknownAgent(t *testing.T) {
	w := newWorld()
	err := w.orch.Resume("ghost", false)
	if err == nil || !strings.Contains(err.Error(), "unknown agent") {
		t.Fatalf("err = %v", err)
	}
}

func TestResumeAllRebuildsStoppedSkipsRunning(t *testing.T) {
	w := newWorld()
	for _, name := range []string{"auth", "tests", "docs"} {
		w.git.branches[name] = true
		w.store.state.Add(state.Agent{Name: name, Provider: "claude", Branch: name, Worktree: "/repo/worktrees/" + name})
	}
	// "auth" is already up; only "tests" and "docs" need rebuilding.
	w.tmux.windows["auth"] = "claude"

	if err := w.orch.ResumeAll(); err != nil {
		t.Fatal(err)
	}
	if !w.tmux.sessions["repo"] {
		t.Error("session should be present")
	}
	for _, name := range []string{"tests", "docs"} {
		if _, ok := w.tmux.windows[name]; !ok {
			t.Errorf("window %s should be recreated", name)
		}
		if got := w.tmux.sent[name]; got != "claude" {
			t.Errorf("%s provider command = %q, want claude", name, got)
		}
	}
	// The already-running agent is never relaunched.
	if _, sent := w.tmux.sent["auth"]; sent {
		t.Error("running agent must not be relaunched")
	}
	if !strings.Contains(w.out.String(), "Resumed 2, 1 already running, 0 failed.") {
		t.Errorf("summary missing or wrong:\n%s", w.out.String())
	}
}

func TestResumeAllEmptyRegistry(t *testing.T) {
	w := newWorld()
	if err := w.orch.ResumeAll(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(w.out.String(), "nothing to resume") {
		t.Errorf("empty registry should report nothing to resume:\n%s", w.out.String())
	}
	if w.tmux.sessions["repo"] {
		t.Error("no session should be created for an empty registry")
	}
}

func TestResumeAllContinuesPastFailure(t *testing.T) {
	w := newWorld()
	// "broken" has a missing worktree and no branch to restore from, so it fails;
	// "auth" and "docs" are healthy and must still be resumed around it.
	w.git.branches["auth"] = true
	w.git.branches["docs"] = true
	w.fs.missing["/repo/worktrees/broken"] = true
	w.store.state.Add(state.Agent{Name: "auth", Provider: "claude", Branch: "auth", Worktree: "/repo/worktrees/auth"})
	w.store.state.Add(state.Agent{Name: "broken", Provider: "claude", Branch: "broken", Worktree: "/repo/worktrees/broken"})
	w.store.state.Add(state.Agent{Name: "docs", Provider: "claude", Branch: "docs", Worktree: "/repo/worktrees/docs"})

	err := w.orch.ResumeAll()
	if err == nil || !strings.Contains(err.Error(), "failed to resume 1 of 3") {
		t.Fatalf("err = %v, want partial-failure summary error", err)
	}
	for _, name := range []string{"auth", "docs"} {
		if _, ok := w.tmux.windows[name]; !ok {
			t.Errorf("healthy agent %s should be resumed despite the failure", name)
		}
	}
	if _, ok := w.tmux.windows["broken"]; ok {
		t.Error("broken agent should not have a window")
	}
	if !strings.Contains(w.out.String(), "Resumed 2, 0 already running, 1 failed.") {
		t.Errorf("summary should reflect the failure:\n%s", w.out.String())
	}
}
