// Package config loads agents configuration from three layers, each
// overriding the previous: built-in defaults, the global config file at
// $XDG_CONFIG_HOME/agents/config.yaml, and .agents.yaml at the repo root.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ProviderConfig describes how to launch one AI provider.
type ProviderConfig struct {
	// Command is the executable to run.
	Command string `yaml:"command"`
	// Args are always passed to the command.
	Args []string `yaml:"args"`
	// PromptArgs are appended when an initial prompt is given; the literal
	// {{prompt}} placeholder is replaced with the prompt text.
	PromptArgs []string `yaml:"promptArgs"`
}

// Config is the fully resolved configuration.
type Config struct {
	DefaultProvider string
	Providers       map[string]ProviderConfig
	// Session is the tmux session name; empty means "derive from repo basename".
	Session string
	// WorktreesRoot is where worktrees live, relative to the repo root unless absolute.
	WorktreesRoot string
	// TemplateDirs is the template search order: repo templates first, then global.
	TemplateDirs  []string
	Notifications bool
}

type fileConfig struct {
	DefaultProvider string                    `yaml:"defaultProvider"`
	Providers       map[string]ProviderConfig `yaml:"providers"`
	Tmux            struct {
		Session string `yaml:"session"`
	} `yaml:"tmux"`
	Worktrees struct {
		Root string `yaml:"root"`
	} `yaml:"worktrees"`
	Templates struct {
		Path string `yaml:"path"`
	} `yaml:"templates"`
	Notifications *bool `yaml:"notifications"`
}

// GlobalDir returns the global configuration directory,
// honoring $XDG_CONFIG_HOME on every platform.
func GlobalDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "agents")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "agents")
}

// Default returns the built-in configuration; the tool is fully usable
// without any config file.
func Default() *Config {
	return &Config{
		DefaultProvider: "claude",
		Providers: map[string]ProviderConfig{
			"claude": {Command: "claude", PromptArgs: []string{"{{prompt}}"}},
			"codex":  {Command: "codex", PromptArgs: []string{"{{prompt}}"}},
			"gemini": {Command: "gemini", PromptArgs: []string{"-i", "{{prompt}}"}},
		},
		WorktreesRoot: "worktrees",
	}
}

// Load builds the resolved configuration for a repository.
func Load(repoRoot string) (*Config, error) {
	cfg := Default()

	globalDir := GlobalDir()
	globalTemplates := filepath.Join(globalDir, "templates")
	repoTemplates := ""

	if globalDir != "" {
		fc, err := readFile(filepath.Join(globalDir, "config.yaml"))
		if err != nil {
			return nil, err
		}
		if fc != nil {
			merge(cfg, *fc)
			if fc.Templates.Path != "" {
				globalTemplates = resolvePath(fc.Templates.Path, globalDir)
			}
		}
	}

	if repoRoot != "" {
		fc, err := readFile(filepath.Join(repoRoot, ".agents.yaml"))
		if err != nil {
			return nil, err
		}
		if fc != nil {
			merge(cfg, *fc)
			if fc.Templates.Path != "" {
				repoTemplates = resolvePath(fc.Templates.Path, repoRoot)
			}
		}
	}

	if repoTemplates != "" {
		cfg.TemplateDirs = append(cfg.TemplateDirs, repoTemplates)
	}
	if globalTemplates != "" {
		cfg.TemplateDirs = append(cfg.TemplateDirs, globalTemplates)
	}
	return cfg, nil
}

func readFile(path string) (*fileConfig, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &fc, nil
}

func merge(dst *Config, src fileConfig) {
	if src.DefaultProvider != "" {
		dst.DefaultProvider = src.DefaultProvider
	}
	for name, p := range src.Providers {
		existing := dst.Providers[name]
		if p.Command != "" {
			existing.Command = p.Command
		}
		if p.Args != nil {
			existing.Args = p.Args
		}
		if p.PromptArgs != nil {
			existing.PromptArgs = p.PromptArgs
		}
		dst.Providers[name] = existing
	}
	if src.Tmux.Session != "" {
		dst.Session = src.Tmux.Session
	}
	if src.Worktrees.Root != "" {
		dst.WorktreesRoot = src.Worktrees.Root
	}
	if src.Notifications != nil {
		dst.Notifications = *src.Notifications
	}
}

func resolvePath(path, base string) string {
	path = expandHome(path)
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(base, path)
}

func expandHome(path string) string {
	if path == "~" || len(path) > 1 && path[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}
