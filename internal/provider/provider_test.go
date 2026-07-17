package provider

import (
	"strings"
	"testing"

	"github.com/joaovictor3g/agents/internal/config"
)

func TestCommandLineNoPrompt(t *testing.T) {
	p := Provider{Name: "claude", Command: "claude", PromptArgs: []string{"{{prompt}}"}}
	got, err := p.CommandLine("")
	if err != nil {
		t.Fatal(err)
	}
	if got != "claude" {
		t.Fatalf("got %q, want %q", got, "claude")
	}
}

func TestCommandLineInjectsPrompt(t *testing.T) {
	p := Provider{Name: "gemini", Command: "gemini", PromptArgs: []string{"-i", "{{prompt}}"}}
	got, err := p.CommandLine("review the auth module")
	if err != nil {
		t.Fatal(err)
	}
	want := `gemini -i 'review the auth module'`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCommandLineQuotesSingleQuotes(t *testing.T) {
	p := Provider{Name: "claude", Command: "claude", PromptArgs: []string{"{{prompt}}"}}
	got, err := p.CommandLine("don't break")
	if err != nil {
		t.Fatal(err)
	}
	want := `claude 'don'\''t break'`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCommandLineWithoutPromptArgsFails(t *testing.T) {
	p := Provider{Name: "custom", Command: "custom"}
	if _, err := p.CommandLine("hi"); err == nil {
		t.Fatal("expected error for provider without promptArgs")
	}
}

func TestCommandLineStaticArgs(t *testing.T) {
	p := Provider{Name: "x", Command: "x", Args: []string{"--yolo"}, PromptArgs: []string{"{{prompt}}"}}
	got, err := p.CommandLine("")
	if err != nil {
		t.Fatal(err)
	}
	if got != "x --yolo" {
		t.Fatalf("got %q", got)
	}
}

func TestRegistryResolve(t *testing.T) {
	r := NewRegistry(config.Default())
	p, err := r.Resolve("claude")
	if err != nil {
		t.Fatal(err)
	}
	if p.Command != "claude" {
		t.Fatalf("got command %q", p.Command)
	}
	if _, err := r.Resolve("nope"); err == nil || !strings.Contains(err.Error(), "claude") {
		t.Fatalf("expected error listing providers, got %v", err)
	}
}

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"simple":      "simple",
		"has space":   "'has space'",
		"a'b":         `'a'\''b'`,
		"multi\nline": "'multi\nline'",
		"./path-ok_1": "./path-ok_1",
	}
	for in, want := range cases {
		if got := ShellQuote(in); got != want {
			t.Errorf("ShellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}
