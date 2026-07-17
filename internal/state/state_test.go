package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingFileYieldsEmpty(t *testing.T) {
	st := &Store{Path: filepath.Join(t.TempDir(), "agents", "state.json")}
	s, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Agents) != 0 {
		t.Fatalf("expected empty state, got %d agents", len(s.Agents))
	}
}

func TestSaveCreatesPrivateFileAndDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "agents")
	st := &Store{Path: filepath.Join(dir, "state.json")}
	if err := st.Save(&State{}); err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat(st.Path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("state file perm = %o, want 600", perm)
	}

	di, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if perm := di.Mode().Perm(); perm != 0o700 {
		t.Errorf("registry dir perm = %o, want 700", perm)
	}
}

func TestRoundTrip(t *testing.T) {
	st := &Store{Path: filepath.Join(t.TempDir(), "agents", "state.json")}
	s := &State{}
	s.Add(Agent{Name: "auth", Provider: "claude", Branch: "auth", Worktree: "/repo/worktrees/auth", CreatedAt: time.Now().UTC()})
	s.Add(Agent{Name: "tests", Provider: "codex", Branch: "tests", Worktree: "/repo/worktrees/tests", CreatedAt: time.Now().UTC()})
	if err := st.Save(s); err != nil {
		t.Fatal(err)
	}

	loaded, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Agents) != 2 {
		t.Fatalf("got %d agents", len(loaded.Agents))
	}
	a, ok := loaded.Get("auth")
	if !ok || a.Provider != "claude" {
		t.Fatalf("Get(auth) = %+v, %v", a, ok)
	}

	if !loaded.Remove("auth") {
		t.Fatal("Remove(auth) reported not found")
	}
	if loaded.Remove("auth") {
		t.Fatal("second Remove(auth) should report not found")
	}
	if _, ok := loaded.Get("auth"); ok {
		t.Fatal("auth still present after Remove")
	}
}
