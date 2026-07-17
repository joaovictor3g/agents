// Command agents orchestrates parallel AI coding agents, each in its own git
// worktree and tmux window.
package main

import (
	"os"

	"github.com/victordias/agents/internal/cli"
)

// version is injected at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(cli.Execute(version))
}
