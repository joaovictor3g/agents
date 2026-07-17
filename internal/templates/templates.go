// Package templates resolves prompt templates. Templates are static markdown
// presets passed to the provider verbatim; no variable substitution happens.
package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Resolve returns the template content. Arguments containing a path
// separator or .md suffix are read as literal file paths; bare names are
// searched as <name>.md through dirs in order.
func Resolve(nameOrPath string, dirs []string) (string, error) {
	if strings.ContainsRune(nameOrPath, os.PathSeparator) || strings.HasSuffix(nameOrPath, ".md") {
		data, err := os.ReadFile(expandHome(nameOrPath))
		if err != nil {
			return "", fmt.Errorf("reading template: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	for _, dir := range dirs {
		data, err := os.ReadFile(filepath.Join(dir, nameOrPath+".md"))
		if err == nil {
			return strings.TrimSpace(string(data)), nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("reading template: %w", err)
		}
	}
	available := Available(dirs)
	if len(available) == 0 {
		return "", fmt.Errorf("template %q not found (no templates in %s)", nameOrPath, strings.Join(dirs, ", "))
	}
	return "", fmt.Errorf("template %q not found (available: %s)", nameOrPath, strings.Join(available, ", "))
}

// Available lists template names found across dirs, sorted and deduplicated.
func Available(dirs []string) []string {
	seen := make(map[string]bool)
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				seen[strings.TrimSuffix(e.Name(), ".md")] = true
			}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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
