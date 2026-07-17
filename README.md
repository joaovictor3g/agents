# agents

**Run a whole team of terminal AI coding agents in parallel on one repo — without them stepping on each other.**

Claude Code, Codex CLI, Gemini CLI, or any CLI you configure. Every agent gets its own **branch**, **worktree**, **tmux window**, and **AI session** — so it feels like managing several engineers on one repo, each with their own checkout. Except every engineer is an AI.

```
$ agents create auth
✔ Created branch auth (from main)
✔ Created worktree /repo/worktrees/auth
✔ Created tmux window myrepo:auth
✔ Started claude

Agent ready. Run agents attach auth to join it.
```

Spin up an entire team in seconds:

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

---

## Contents

- [Requirements](#requirements)
- [Installation](#installation)
- [Commands](#commands) — [create](#agents-create-name) · [plan](#agents-plan-request) · [spawn](#agents-spawn-planmd) · [list](#agents-list-alias-ls) · [attach](#agents-attach-name) · [delete](#agents-delete-name-alias-rm) · [merge](#agents-merge-name) · [status](#agents-status) · [watch](#agents-watch)
- [Configuration](#configuration)
- [Prompt templates](#prompt-templates)
- [How it works](#how-it-works)
- [Development](#development)
- [License](#license)

---

## Requirements

- macOS (developed and tested here) or Linux (supported, unit-tested in CI, not yet exercised end-to-end)
- `git` 2.20+, `tmux`, and at least one AI CLI on your `PATH` (`claude`, `codex`, `gemini`, …)

Works from any terminal — Ghostty, iTerm2, Terminal.app — because tmux is the substrate, not the launcher. Creating an agent never steals focus; you join it with `agents attach`.

## Installation

**Homebrew** (macOS + Linux, installs `tmux` automatically):

```sh
brew install joaovictor3g/tap/agents
```

`agents` ships as a Homebrew *cask*, so use the full `tap/agents` path — `brew install agents` alone won't find it.

**Go** (needs `tmux` on your `PATH` separately):

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
| `-t, --template <name\|path>` | Prompt template injected at startup. A bare name resolves to `<name>.md` in your template dirs; a path or `.md` suffix is read as a file. |
| `--prompt <text>` | Inline prompt injected at startup. Mutually exclusive with `--template`. |
| `-b, --base <ref>` | Base ref for the new branch. Default: the repo's default branch. Stack agents with `--base auth`. |
| `-a, --attach` | Jump to the agent's window after creation. |

Worth knowing:

- **Branches are based on the repo's default branch** (resolved from `origin/HEAD`, falling back to `main`/`master`) — never "wherever HEAD happens to be". No implicit fetch or pull.
- **An existing branch is adopted**, making `delete` → `create` a pause/resume cycle. If it's checked out elsewhere, `create` fails with a clear error.
- **Prompts are injected as command-line arguments** (e.g. `claude 'your prompt'`), never typed into a running TUI — so there's no startup race.
- Names must be flat: letters, digits, `.`, `_`, `-`. No slashes.
- The worktree root is added to `.git/info/exclude` automatically — worktrees never pollute `git status`, and your tracked `.gitignore` is never touched.

### `agents plan <request>`

Asks a provider to decompose a high-level feature request into a small team of specialized agents, and emits the result as a Markdown plan. **It only generates a plan — it never creates agents.** Review (and edit) the plan, then hand it to `agents spawn` to create the team.

```sh
agents plan "Implement Stripe subscriptions" > plan.md
# review / edit plan.md
agents spawn plan.md
```

The output is exactly the format `agents spawn` reads — one `## <name>` heading per agent, with `- <task>` bullets:

```md
## auth
- OAuth login and session handling

## payments
- Stripe billing and webhooks

## tests
- End-to-end coverage for the new flows
```

| Flag | Description |
|---|---|
| `-p, --provider <name>` | Provider to plan with. Default: `defaultProvider` from config. |
| `-o, --out <file>` | Write the plan to a file instead of stdout. |

Worth knowing:

- **Planning runs the provider in headless mode** (e.g. `claude -p`, `codex exec`), configured per provider via `planArgs` — the headless counterpart to `promptArgs`. A provider with no `planArgs` can't be used for planning and says so.
- **The plan must parse.** Its output is validated through the same parser `spawn` uses; if the model wraps it in prose or a code fence, `plan` strips the fence and retries once before giving up with the raw output shown.
- With no `--out` the plan goes to stdout with no other chatter, so `agents plan … > plan.md` captures just the plan.

### `agents spawn <plan.md>`

Creates a whole team of agents in one shot from a Markdown plan file, dispatching each one's initial task. Every agent is provisioned through the same path as `agents create` — branch, worktree, tmux window, provider, and injected prompt.

The plan is one second-level heading per agent (the agent name), with bullet lines as that agent's task(s):

```md
## auth
- OAuth integration

## payments
- Stripe billing

## tests
- Playwright coverage
```

A level-1 title and any non-bullet prose are ignored; bullets under a heading are joined into that agent's task prompt. Agent names follow the same rules as `create` (letters, digits, `.`, `_`, `-`).

Worth knowing:

- **Each agent uses the default provider** (`defaultProvider` from config). Per-agent provider/template selection is not part of the v1 plan format.
- **Spawning continues past a failure**: agents created before it keep running, the rest are still attempted, and a `N created, K failed` summary is printed. The command exits non-zero if any agent failed.
- The plan is rejected up front if it has no headings, a duplicate agent name, an invalid name, or an agent with no tasks.

### `agents list` (alias: `ls`)

Every agent with its live status:

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

Clean up `dead` and `broken` agents with `agents delete <name>`.

### `agents attach <name>`

Switches to the agent's tmux window. Inside tmux it uses `switch-client` (seamless, no nesting); outside tmux it attaches to the session.

### `agents resume [name]`

Brings a stopped agent back to life. A reboot (or `tmux kill-server`) destroys the tmux window and the AI process, but the branch, worktree, and registry entry all survive — so there is no need to create a new agent, which would only spawn a new branch.

`resume` rebuilds the missing pieces **in place**, reusing the same branch and worktree: it re-adds the worktree if its directory is gone, recreates the tmux window, and relaunches the provider. It is **idempotent** — an agent whose window is still alive is simply re-attached, never duplicated.

Pass a single agent name, or `--all` to recover every registered agent at once after a reboot. Exactly one of the two is required — combining a name with `--all` is an error.

| Flag | Description |
|---|---|
| `-a, --attach` | Jump to the agent's window after resuming (single-agent only). |
| `--all` | Resume every registered agent. Already-running agents are skipped, failures are reported, and the run continues to a summary rather than aborting on the first error. |

```console
$ agents resume auth          # after a reboot
✓ Recreated tmux window myrepo:auth
✓ Restarted claude

Agent resumed. Run agents attach auth to join it.
```

```console
$ agents resume --all         # recover the whole team at once
✓ Recreated tmux window myrepo:tests
✓ Restarted claude
Agent auth is already running.
✓ Recreated tmux window myrepo:docs
✓ Restarted claude

Resumed 2, 1 already running, 0 failed.
```

The AI conversation itself is not restored (that state lives in the provider, not in `agents`); the provider launches fresh, and you can use its own resume flag (e.g. `claude --continue`) from inside the window if you need the prior session.

### `agents delete <name>` (alias: `rm`)

Stops the AI session and removes the tmux window and worktree.

| Flag | Description |
|---|---|
| `--branch` | Also delete the agent's branch. **Kept by default** — it may hold the agent's only unmerged work. |
| `-f, --force` | Discard uncommitted worktree changes; with `--branch`, also delete an unmerged branch. |

**Idempotent**: pieces you already removed by hand are treated as done, so a half-broken agent always cleans up with the same command. Without `--force`, a dirty worktree or unmerged branch refuses to die.

### `agents merge <name>`

Merges the agent's branch into the repo's default branch, then tears the agent down.

| Flag | Description |
|---|---|
| `--keep-branch` | Keep the branch after merging. **Deleted by default.** |
| `-f, --force` | Merge even with uncommitted changes in the agent's worktree (left behind and removed with the worktree). |

Safety model:

- **Aborts before touching anything** if the main checkout or the agent's worktree has uncommitted changes (AI agents often leave work uncommitted — merging silently would lose it), or if a merge is already in progress.
- **On conflict, the merge is left in progress** in the main checkout and *nothing* is torn down — agent, window, worktree, and branch all survive. Resolve, commit, then `agents delete <name>`.
- Teardown happens only after a fully successful merge.

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

### `agents doctor`

Scans every registered agent and the repository for common problems and prints an actionable fix for each — a missing worktree, a dead tmux window, a provider binary that is no longer on your `PATH`, a detached HEAD, a merge left in progress, or uncommitted changes:

```
! auth: worktree directory is missing
  → run `agents resume auth`
! review: provider command "claude" is not on PATH
  → install claude or fix your PATH
```

- Read-only: it never changes anything, so it is always safe to run.
- Exits **non-zero** when any problem is found and prints `✔ No problems found. All agents are healthy.` (exit `0`) otherwise, so it drops straight into scripts and CI.

### `agents watch`

A **read-only dashboard** that mirrors every agent side by side in a tiled grid, so you can watch the whole team at once:

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

- The panes are **mirrors** — the agents' real windows are never touched. To interact, use `agents attach <name>`.
- Reconciles on every run: create or delete agents, run `agents watch` again, and the grid re-syncs.
- Lives in a reserved `_watch` tmux window; dismiss it by closing the window (`Ctrl-b &`). Nothing to clean up.

## Configuration

Layered — later layers override earlier ones:

1. **Built-in defaults** — fully usable with zero config files.
2. **Global**: `~/.config/agents/config.yaml` (respects `$XDG_CONFIG_HOME`; same path on macOS and Linux).
3. **Repo**: `.agents.yaml` at the repo root — commit it to share team conventions.
4. **Command-line flags** override everything.

Full reference (every key is optional):

```yaml
# Provider used when --provider is not given.
defaultProvider: claude

# Providers merge by name across layers: override one field of a
# built-in provider, or add entirely new ones.
providers:
  claude:
    command: claude              # executable to launch
    args: []                     # arguments always passed
    promptArgs: ["{{prompt}}"]   # appended when a prompt is injected;
                                 # {{prompt}} is replaced with the text
    planArgs: ["-p", "{{prompt}}"] # headless invocation for `agents plan`
                                 # (must print to stdout and exit)
  codex:
    command: codex
    promptArgs: ["{{prompt}}"]
    planArgs: ["exec", "{{prompt}}"]
  gemini:
    command: gemini
    promptArgs: ["-i", "{{prompt}}"]
    planArgs: ["-p", "{{prompt}}"]
  aider:                         # example custom provider
    command: aider
    promptArgs: ["--message", "{{prompt}}"]

tmux:
  # Session holding all agent windows. Default: the repo dir's basename.
  session: myproject

worktrees:
  # Where worktrees live; relative to the repo root, or absolute.
  root: worktrees/

templates:
  # Template directory. In global config this replaces the default;
  # in .agents.yaml it adds a repo-level dir that is searched first.
  path: ~/.config/agents/templates

# Desktop notification when an agent is ready or a merge completes
# (osascript on macOS, notify-send on Linux). Default: false.
notifications: true
```

A provider without `promptArgs` still works — but `--template`/`--prompt` will fail with a clear error rather than gamble on typing into its TUI. Likewise, a provider without `planArgs` works for everything except `agents plan`, which needs a headless (print-to-stdout) invocation.

## Prompt templates

Plain markdown files passed to the provider verbatim (no variable substitution):

```
~/.config/agents/templates/
  reviewer.md
  architect.md
  tests.md
```

```sh
agents create review --template reviewer          # searches repo dir, then global
agents create docs --template ./prompts/docs.md   # explicit path
```

With `templates.path` set in `.agents.yaml`, a team can commit repo-specific prompts, and they win over global ones of the same name.

## How it works

- **Registry**: each agent's identity (name, provider, branch, worktree) lives in `.git/agents/state.json` — per-clone, never committable, shared correctly across worktrees. Liveness is *never* stored: `list`/`status` reconcile against live tmux windows and the filesystem on every read, so the registry can't lie about what's running.
- **One tmux session per repository** (named after the repo directory by default). Windows are created detached; `create` never yanks your terminal.
- **Windows wrap your shell**: the provider runs inside your login shell, so when it exits the window survives (`idle`) — poke around the worktree, or relaunch.
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
