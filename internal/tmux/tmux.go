// Package tmux wraps the tmux CLI. Windows are always targeted by exact name
// (session:=window) so similarly named windows never collide.
package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/victordias/agents/internal/execx"
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
