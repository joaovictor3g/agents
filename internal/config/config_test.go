package config

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestZeroConfigDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultProvider != "claude" {
		t.Errorf("DefaultProvider = %q", cfg.DefaultProvider)
	}
	if cfg.WorktreesRoot != "worktrees" {
		t.Errorf("WorktreesRoot = %q", cfg.WorktreesRoot)
	}
	if len(cfg.Providers) != 3 {
		t.Errorf("expected 3 built-in providers, got %d", len(cfg.Providers))
	}
}

func TestLayeringRepoOverridesGlobal(t *testing.T) {
	global := t.TempDir()
	repo := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", global)

	write(t, filepath.Join(global, "agents", "config.yaml"), `
defaultProvider: codex
tmux:
  session: global-session
notifications: true
`)
	write(t, filepath.Join(repo, ".agents.yaml"), `
tmux:
  session: repo-session
`)

	cfg, err := Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultProvider != "codex" {
		t.Errorf("DefaultProvider = %q, want global value codex", cfg.DefaultProvider)
	}
	if cfg.Session != "repo-session" {
		t.Errorf("Session = %q, want repo override", cfg.Session)
	}
	if !cfg.Notifications {
		t.Error("Notifications should survive from global layer")
	}
}

func TestProviderMergeByField(t *testing.T) {
	global := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", global)
	write(t, filepath.Join(global, "agents", "config.yaml"), `
providers:
  claude:
    command: /opt/bin/claude
  aider:
    command: aider
    promptArgs: ["--message", "{{prompt}}"]
`)

	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	claude := cfg.Providers["claude"]
	if claude.Command != "/opt/bin/claude" {
		t.Errorf("claude.Command = %q", claude.Command)
	}
	if len(claude.PromptArgs) != 1 || claude.PromptArgs[0] != "{{prompt}}" {
		t.Errorf("claude.PromptArgs lost built-in default: %v", claude.PromptArgs)
	}
	if _, ok := cfg.Providers["aider"]; !ok {
		t.Error("new provider from config missing")
	}
}

func TestTemplateDirsRepoFirst(t *testing.T) {
	global := t.TempDir()
	repo := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", global)
	write(t, filepath.Join(repo, ".agents.yaml"), `
templates:
  path: prompts
`)

	cfg, err := Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.TemplateDirs) != 2 {
		t.Fatalf("TemplateDirs = %v", cfg.TemplateDirs)
	}
	if cfg.TemplateDirs[0] != filepath.Join(repo, "prompts") {
		t.Errorf("repo templates dir should come first: %v", cfg.TemplateDirs)
	}
	if cfg.TemplateDirs[1] != filepath.Join(global, "agents", "templates") {
		t.Errorf("global templates dir wrong: %v", cfg.TemplateDirs)
	}
}
