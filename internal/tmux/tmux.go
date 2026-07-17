// Package tmux wraps the tmux CLI. Windows are always targeted by exact name
// (session:=window) so similarly named windows never collide.
package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/joaovictor3g/agents/internal/execx"
)

// Client runs tmux commands.
type Client struct {
	run execx.Runner
}

// New returns a tmux client.
func New(run execx.Runner) *Client { return &Client{run: run} }

// Installed reports whether the tmux binary is available.
func (c *Client) Installed() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

// InsideTmux reports whether the current process runs inside a tmux client.
func (c *Client) InsideTmux() bool { return os.Getenv("TMUX") != "" }

func target(session, window string) string {
	return fmt.Sprintf("=%s:=%s", session, window)
}

// HasSession reports whether a session with the exact name exists.
func (c *Client) HasSession(name string) bool {
	_, err := c.run.Output("tmux", "has-session", "-t", "="+name)
	return err == nil
}

// NewSession creates a detached session whose first window starts in dir.
func (c *Client) NewSession(name, dir string) error {
	_, err := c.run.Output("tmux", "new-session", "-d", "-s", name, "-c", dir)
	return err
}

// NewWindow creates a window in session without changing focus.
func (c *Client) NewWindow(session, name, dir string) error {
	_, err := c.run.Output("tmux", "new-window", "-d", "-t", "="+session+":", "-n", name, "-c", dir)
	return err
}

// KillWindow kills a window by exact name.
func (c *Client) KillWindow(session, name string) error {
	_, err := c.run.Output("tmux", "kill-window", "-t", target(session, name))
	return err
}

// Windows returns window name -> current foreground command for every window
// in the session, in one tmux call. A missing session yields an empty map.
func (c *Client) Windows(session string) (map[string]string, error) {
	out, err := c.run.Output("tmux", "list-panes", "-s", "-t", "="+session,
		"-F", "#{window_name}\t#{pane_current_command}")
	if err != nil {
		if !c.HasSession(session) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	windows := make(map[string]string)
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		name, cmd, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		if _, seen := windows[name]; !seen {
			windows[name] = cmd
		}
	}
	return windows, nil
}

// SendCommand types a shell command into a window and presses Enter.
func (c *Client) SendCommand(session, window, command string) error {
	_, err := c.run.Output("tmux", "send-keys", "-t", target(session, window), command, "Enter")
	return err
}

// Attach focuses the window: switch-client when already inside tmux,
// otherwise an interactive attach-session.
func (c *Client) Attach(session, window string) error {
	if _, err := c.run.Output("tmux", "select-window", "-t", target(session, window)); err != nil {
		return err
	}
	if c.InsideTmux() {
		_, err := c.run.Output("tmux", "switch-client", "-t", "="+session)
		return err
	}
	return c.run.Interactive("tmux", "attach-session", "-t", "="+session)
}

// PanesInWindow returns pane id -> pane title for every pane in the window,
// used to reconcile the watch dashboard. A missing window yields an empty map.
func (c *Client) PanesInWindow(session, window string) (map[string]string, error) {
	out, err := c.run.Output("tmux", "list-panes", "-t", target(session, window),
		"-F", "#{pane_id}\t#{pane_title}")
	if err != nil {
		return map[string]string{}, nil
	}
	panes := make(map[string]string)
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		id, title, ok := strings.Cut(line, "\t")
		if ok {
			panes[id] = title
		}
	}
	return panes, nil
}

// NewWindowRunning creates a detached window running command and returns the
// id of its pane.
func (c *Client) NewWindowRunning(session, window, command string) (string, error) {
	return c.run.Output("tmux", "new-window", "-d", "-P", "-F", "#{pane_id}",
		"-t", "="+session+":", "-n", window, command)
}

// SplitWindow splits the window with a new pane running command and returns
// the new pane's id.
func (c *Client) SplitWindow(session, window, command string) (string, error) {
	return c.run.Output("tmux", "split-window", "-d", "-P", "-F", "#{pane_id}",
		"-t", target(session, window), command)
}

// KillPane kills a single pane by id.
func (c *Client) KillPane(paneID string) error {
	_, err := c.run.Output("tmux", "kill-pane", "-t", paneID)
	return err
}

// SetPaneTitle tags a pane with a title, used as the reconciliation key.
func (c *Client) SetPaneTitle(paneID, title string) error {
	_, err := c.run.Output("tmux", "select-pane", "-t", paneID, "-T", title)
	return err
}

// SelectLayout applies a named layout (e.g. "tiled") to the window.
func (c *Client) SelectLayout(session, window, layout string) error {
	_, err := c.run.Output("tmux", "select-layout", "-t", target(session, window), layout)
	return err
}

// CapturePane returns the visible text of the target window's pane.
func (c *Client) CapturePane(session, window string) (string, error) {
	return c.run.Output("tmux", "capture-pane", "-p", "-t", target(session, window))
}

// PaneSize returns the width and height of the pane the caller runs in,
// resolved via $TMUX_PANE. It falls back to 80x24 outside tmux.
func (c *Client) PaneSize() (width, height int) {
	pane := os.Getenv("TMUX_PANE")
	if pane == "" {
		return 80, 24
	}
	out, err := c.run.Output("tmux", "display-message", "-p", "-t", pane, "-F", "#{pane_width}\t#{pane_height}")
	if err != nil {
		return 80, 24
	}
	w, h, ok := strings.Cut(out, "\t")
	if !ok {
		return 80, 24
	}
	width, _ = strconv.Atoi(w)
	height, _ = strconv.Atoi(h)
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	return width, height
}
