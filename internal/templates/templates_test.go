package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveByNameSearchesInOrder(t *testing.T) {
	repo := t.TempDir()
	global := t.TempDir()
	os.WriteFile(filepath.Join(repo, "reviewer.md"), []byte("repo reviewer"), 0o644)
	os.WriteFile(filepath.Join(global, "reviewer.md"), []byte("global reviewer"), 0o644)
	os.WriteFile(filepath.Join(global, "docs.md"), []byte("global docs"), 0o644)

	got, err := Resolve("reviewer", []string{repo, global})
	if err != nil {
		t.Fatal(err)
	}
	if got != "repo reviewer" {
		t.Fatalf("got %q, want repo version to win", got)
	}

	got, err = Resolve("docs", []string{repo, global})
	if err != nil {
		t.Fatal(err)
	}
	if got != "global docs" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveByPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.md")
	os.WriteFile(path, []byte("  custom prompt\n"), 0o644)

	got, err := Resolve(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "custom prompt" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveMissingListsAvailable(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "frontend.md"), []byte("x"), 0o644)

	_, err := Resolve("nope", []string{dir})
	if err == nil || !strings.Contains(err.Error(), "frontend") {
		t.Fatalf("expected error listing available templates, got %v", err)
	}
}
