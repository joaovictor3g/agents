# agents

Orchestrate a team of terminal AI coding agents — Claude Code, Codex CLI, Gemini CLI, or any CLI you configure — working **in parallel on the same repository** without stepping on each other.

Every agent owns exactly one:

- **git branch**
- **git worktree**
- **tmux window**
- **AI terminal session**

Agents never share a worktree, so the workflow feels like managing several engineers on one repo — each with their own checkout — except every engineer is an AI.

```
$ agents create auth
✔ Created branch auth (from main)
✔ Created worktree /repo/worktrees/auth
✔ Created tmux window myrepo:auth
✔ Started claude

Agent ready. Run agents attach auth to join it.
```

Within seconds you can have an entire team running:

```
$ agents create auth
$ agents create tests --provider codex
$ agents create review --template reviewer
$ agents list
NAME     STATUS    PROVIDER   BRANCH   WORKTREE
auth     running   claude     auth     /repo/worktrees/auth
tests    running   codex      tests    /repo/worktrees/tests
review   running   claude     review   /repo/worktrees/review
```

## Requirements

- macOS (developed and tested here) or Linux (supported, unit-tested in CI, not yet exercised end-to-end)
- `git` 2.20+
- `tmux`
- at least one AI CLI on your `PATH` (`claude`, `codex`, `gemini`, …)

`agents` works from any terminal — Ghostty, iTerm2, Terminal.app — because tmux is the substrate, not the launcher. Creating an agent never steals focus; you join it with `agents attach`.

## Installation

**Homebrew** (macOS and Linux — installs `tmux` automatically):

```sh
brew install joaovictor3g/tap/agents
```

`agents` ships as a Homebrew *cask*, so use the full `tap/agents` path above.
`brew install agents` on its own will not find it.

**Go** (requires `tmux` on your `PATH` separately):

```sh
go install github.com/joaovictor3g/agents/cmd/agents@latest
```

**From source:**

```sh
make build        # ./bin/agents
make install      # $GOBIN/agents
```

## Commands

### `agents create <name>`

Creates the agent's branch (or adopts an existing one), its worktree, a tmux window, and launches the AI provider inside it.

| Flag | Description |
|---|---|
| `-p, --provider <name>` | AI provider to launch. Default: `defaultProvider` from config (`claude` out of the box). |
| `-t, --template <name\|path>` | Prompt template injected at startup. A bare name resolves to `<name>.md` in your template directories; a path or `.md` suffix is read as a file. |
| `--prompt <text>` | Inline prompt injected at startup. Mutually exclusive with `--template`. |
| `-b, --base <ref>` | Base ref for the new branch. Default: the repo's default branch. Use this to stack agents (`agents create auth-tests --base auth`). |
| `-a, --attach` | Jump to the agent's window after creation. |

Behavior worth knowing:

- **Branches are based on the repo's default branch** (resolved from `origin/HEAD`, falling back to `main`/`master`) — never on "wherever HEAD happens to be". No implicit fetch or pull is ever performed.
- **If the branch already exists it is adopted**, making `delete` → `create` a pause/resume cycle. If it's checked out in another worktree, `create` fails with a clear error.
- **Prompts are injected as command-line arguments** at launch (e.g. `claude 'your prompt'`), never typed into a running TUI — so there is no startup race.
- Names must be flat: letters, digits, `.`, `_`, `-`. No slashes.
- The worktree root is added to `.git/info/exclude` automatically, so worktrees never pollute `git status` and your tracked `.gitignore` is never touched.

### `agents list` (alias: `ls`)

Shows every agent with its live status:

```
NAME     STATUS    PROVIDER   BRANCH   WORKTREE
auth     running   claude     auth     /repo/worktrees/auth
tests    idle      codex      tests    /repo/worktrees/tests
```

| Status | Meaning |
|---|---|
| `running` | the provider process is active in the window |
| `idle` | the window is alive but the provider exited to a shell |
| `dead` | the tmux window is gone (killed, or the tmux server restarted) |
| `broken` | the worktree directory is missing |

`dead` and `broken` agents are cleaned up with `agents delete <name>`.

### `agents attach <name>`

Switches to the agent's tmux window. Inside tmux it uses `switch-client` (seamless, no nesting); outside tmux it attaches to the session.

### `agents delete <name>` (alias: `rm`)

Stops the AI session and removes the tmux window and worktree.

| Flag | Description |
|---|---|
| `--branch` | Also delete the agent's branch. **The branch is kept by default** — it may hold the agent's only unmerged work. |
| `-f, --force` | Discard uncommitted worktree changes; with `--branch`, also delete an unmerged branch. |

Deletion is **idempotent**: pieces you already removed by hand are treated as done, so a half-broken agent can always be cleaned up with the same command. Without `--force`, a dirty worktree or unmerged branch refuses to die.

### `agents merge <name>`

Merges the agent's branch into the repo's default branch, then tears the agent down.

| Flag | Description |
|---|---|
| `--keep-branch` | Keep the branch after merging. **By default the merged branch is deleted.** |
| `-f, --force` | Merge even if the agent's worktree has uncommitted changes (they are left behind and removed with the worktree). |

Safety model:

- **Aborts before touching anything** if the main checkout has uncommitted changes, if the agent's worktree has uncommitted changes (AI agents often leave work uncommitted — merging silently without it would lose output), or if a merge is already in progress.
- **On conflict, the merge is left in progress** in the main checkout for you to resolve — and *nothing* is torn down: the agent, window, worktree, and branch all survive. Resolve, commit, then run `agents delete <name>`.
- Teardown (window, worktree, branch, registry entry) happens only after a fully successful merge.

### `agents status`

Repository-level health plus the agent table:

```
Repository:  /Users/you/code/myrepo
Branch:      main
Default:     main
Session:     myrepo

NAME     STATUS    PROVIDER   BRANCH   FIX
auth     running   claude     auth
old      dead      claude     old      agents delete old
```

Warns on detached HEAD and merges left in progress.

### `agents watch`

Opens a **read-only dashboard** that mirrors every agent side by side in a
tiled grid, so you can watch the whole team at once:

```
┌ auth ─────────────┐ ┌ tests ────────────┐
│ (claude working…) │ │ (codex working…)  │
└───────────────────┘ └───────────────────┘
┌ review ───────────┐
│ (claude working…) │
└───────────────────┘
```

| Flag | Description |
|---|---|
| `-i, --interval <dur>` | Refresh interval for each mirror pane. Default: `2s`. |

- The panes are **mirrors**: the agents' real windows are never touched. To
  interact with an agent, use `agents attach <name>`.
- The dashboard reconciles on every run — create or delete agents, then run
  `agents watch` again and the grid re-syncs (new panes added, gone agents
  removed).
- It lives in a reserved `_watch` tmux window; dismiss it by closing the
  window (`Ctrl-b &`). Nothing to clean up.

## Configuration

Configuration is layered; later layers override earlier ones:

1. **Built-in defaults** — the tool is fully usable with zero config files.
2. **Global**: `~/.config/agents/config.yaml` (respects `$XDG_CONFIG_HOME`, same path on macOS and Linux).
3. **Repo**: `.agents.yaml` at the repository root — commit it to share team conventions.
4. Command-line flags override everything.

Full reference (every key is optional):

```yaml
# Provider used when --provider is not given.
defaultProvider: claude

# Providers merge by name across layers: override one field of a built-in
# provider, or add entirely new ones.
providers:
  claude:
    command: claude          # executable to launch
    args: []                 # arguments always passed
    promptArgs: ["{{prompt}}"]   # appended when a prompt is injected;
                                 # {{prompt}} is replaced with the text
  codex:
    command: codex
    promptArgs: ["{{prompt}}"]
  gemini:
    command: gemini
    promptArgs: ["-i", "{{prompt}}"]
  # example of a custom provider:
  aider:
    command: aider
    promptArgs: ["--message", "{{prompt}}"]

tmux:
  # Session that holds all agent windows.
  # Default: the repository directory's basename.
  session: myproject

worktrees:
  # Where worktrees live; relative to the repo root, or absolute.
  root: worktrees/

templates:
  # Template directory. In the global config this replaces
  # ~/.config/agents/templates; in .agents.yaml it adds a repo-level
  # directory that is searched first.
  path: ~/.config/agents/templates

# Desktop notification when an agent is ready or a merge completes
# (osascript on macOS, notify-send on Linux). Default: false.
notifications: true
```

A provider without `promptArgs` still works — but `--template`/`--prompt` will fail with a clear error rather than gamble on typing into its TUI.

## Prompt templates

Templates are plain markdown files passed to the provider verbatim (no variable substitution):

```
~/.config/agents/templates/
  reviewer.md
  architect.md
  tests.md
```

```sh
agents create review --template reviewer     # searches repo dir, then global
agents create docs --template ./prompts/docs.md   # explicit path
```

With `templates.path` set in `.agents.yaml`, a team can commit repo-specific prompts and they win over global ones of the same name.

## How it works

- **Registry**: each agent's identity (name, provider, branch, worktree) is recorded in `.git/agents/state.json` — per-clone, never committable, shared correctly across worktrees. Liveness is *never* stored: `list`/`status` reconcile the registry against live tmux windows and the filesystem on every read, so the registry cannot lie about what's running.
- **One tmux session per repository** (named after the repo directory by default). Windows are created detached; `create` never yanks your terminal.
- **Windows wrap your shell**: the provider runs inside your login shell, so when it exits the window survives (that's `idle`), you can poke around the worktree, or relaunch.
- **State-changing git commands run only where they say they do**: `merge` is the only command that touches your main checkout, and only behind its preflight guards.

## Development

```sh
make build     # build ./bin/agents
make test      # go test ./...
make lint      # gofmt + go vet
```

The orchestrator talks to git and tmux only through interfaces; all policy (guards, teardown ordering, idempotence, status reconciliation) is unit-tested with fakes, and the git wrapper additionally has integration tests against a real temp repository. tmux is never required to run the test suite.

## License

MIT
