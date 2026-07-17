package ui

import (
	"strings"
	"testing"
)

func lines(s string) []string { return strings.Split(s, "\n") }

func TestRenderFramePadsToWidthAndHeight(t *testing.T) {
	got := RenderFrame("auth", "hello", 10, 3)
	ls := lines(got)
	if len(ls) != 2 { // header + one body line (body has one line)
		t.Fatalf("got %d lines: %q", len(ls), ls)
	}
	for _, l := range ls {
		if len([]rune(l)) != 10 {
			t.Errorf("line %q width = %d, want 10", l, len([]rune(l)))
		}
	}
	if !strings.HasPrefix(ls[0], "auth") {
		t.Errorf("header = %q", ls[0])
	}
}

func TestRenderFrameTruncatesWideLines(t *testing.T) {
	got := RenderFrame("h", "this-line-is-far-too-wide", 8, 2)
	body := lines(got)[1]
	if len([]rune(body)) != 8 {
		t.Fatalf("body width = %d, want 8", len([]rune(body)))
	}
	if body != "this-lin" {
		t.Errorf("body = %q, want truncation to 8 runes", body)
	}
}

func TestRenderFrameTailsToHeight(t *testing.T) {
	body := "l1\nl2\nl3\nl4\nl5"
	got := RenderFrame("head", body, 4, 3) // header + 2 body rows
	ls := lines(got)
	if len(ls) != 3 {
		t.Fatalf("got %d lines, want 3: %q", len(ls), ls)
	}
	// Should show the LAST two body lines (tail), not the first two.
	if strings.TrimSpace(ls[1]) != "l4" || strings.TrimSpace(ls[2]) != "l5" {
		t.Errorf("tail wrong: %q", ls[1:])
	}
}

func TestRenderFrameHandlesTinyPane(t *testing.T) {
	got := RenderFrame("header", "body", 0, 0)
	if got == "" {
		t.Fatal("expected at least a clamped header line")
	}
	if strings.Contains(got, "\n") {
		t.Errorf("height clamped to 1 should yield a single line, got %q", got)
	}
}
