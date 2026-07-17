// Package platform isolates the small amount of OS-specific behavior so
// adding a platform never touches the orchestrator.
package platform

import (
	"os/exec"
	"runtime"
	"strings"

	"github.com/joaovictor3g/agents/internal/execx"
)

// Notifier posts desktop notifications.
type Notifier interface {
	Notify(title, message string)
}

// NewNotifier returns the best notifier for the current platform:
// osascript on macOS, notify-send on Linux, otherwise a no-op.
func NewNotifier(run execx.Runner) Notifier {
	switch runtime.GOOS {
	case "darwin":
		return macNotifier{run: run}
	case "linux":
		if _, err := exec.LookPath("notify-send"); err == nil {
			return linuxNotifier{run: run}
		}
	}
	return noopNotifier{}
}

type macNotifier struct{ run execx.Runner }

func (n macNotifier) Notify(title, message string) {
	script := `display notification "` + escapeAppleScript(message) + `" with title "` + escapeAppleScript(title) + `"`
	_, _ = n.run.Output("osascript", "-e", script)
}

func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `"`, `\"`)
}

type linuxNotifier struct{ run execx.Runner }

func (n linuxNotifier) Notify(title, message string) {
	_, _ = n.run.Output("notify-send", title, message)
}

type noopNotifier struct{}

func (noopNotifier) Notify(string, string) {}
