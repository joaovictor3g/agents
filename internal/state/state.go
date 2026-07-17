// Package state persists the agent registry. The registry is the source of
// truth for agent identity (name, provider, branch, worktree); liveness is
// always derived from git and tmux at read time.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Agent is one registered agent.
type Agent struct {
	Name      string    `json:"name"`
	Provider  string    `json:"provider"`
	Branch    string    `json:"branch"`
	Worktree  string    `json:"worktree"`
	CreatedAt time.Time `json:"createdAt"`
}

// State is the full registry.
type State struct {
	Agents []Agent `json:"agents"`
}

// Get returns the agent with the given name.
func (s *State) Get(name string) (Agent, bool) {
	for _, a := range s.Agents {
		if a.Name == name {
			return a, true
		}
	}
	return Agent{}, false
}

// Add appends an agent to the registry.
func (s *State) Add(a Agent) {
	s.Agents = append(s.Agents, a)
}

// Remove deletes the agent with the given name, reporting whether it existed.
func (s *State) Remove(name string) bool {
	for i, a := range s.Agents {
		if a.Name == name {
			s.Agents = append(s.Agents[:i], s.Agents[i+1:]...)
			return true
		}
	}
	return false
}

// Store reads and writes the registry file.
type Store struct {
	// Path is the registry file location, e.g. <git-common-dir>/agents/state.json.
	Path string
}

// Load reads the registry; a missing file yields an empty registry.
func (st *Store) Load() (*State, error) {
	data, err := os.ReadFile(st.Path)
	if os.IsNotExist(err) {
		return &State{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading agent registry: %w", err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing agent registry %s: %w", st.Path, err)
	}
	return &s, nil
}

// Save writes the registry atomically. The registry holds only local metadata,
// but it is per-user tool state, so the directory and file are created private
// (0700/0600) rather than world-readable.
func (st *Store) Save(s *State) error {
	if err := os.MkdirAll(filepath.Dir(st.Path), 0o700); err != nil {
		return fmt.Errorf("creating registry directory: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := st.Path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("writing agent registry: %w", err)
	}
	if err := os.Rename(tmp, st.Path); err != nil {
		return fmt.Errorf("writing agent registry: %w", err)
	}
	return nil
}
