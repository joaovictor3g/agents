// Package execx is the single seam through which agents shells out to
// external tools (git, tmux, osascript). Every package that runs commands
// depends on Runner, so behavior is testable with fakes.
package execx

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Runner executes external commands.
type Runner interface {
	// Output runs a command and returns its trimmed stdout.
	Output(name string, args ...string) (string, error)
	// Interactive runs a command wired to the current terminal's stdio.
	Interactive(name string, args ...string) error
}

// Error describes a failed command, preserving stderr for user-facing messages.
type Error struct {
	Name     string
	Args     []string
	Stderr   string
	ExitCode int
	Err      error
}

func (e *Error) Error() string {
	msg := fmt.Sprintf("%s %s: %v", e.Name, strings.Join(e.Args, " "), e.Err)
	if e.Stderr != "" {
		msg += ": " + e.Stderr
	}
	return msg
}

func (e *Error) Unwrap() error { return e.Err }

// ExitCode returns the command's exit code from err, or -1 if err is not an
// execx.Error.
func ExitCode(err error) int {
	var xerr *Error
	if errors.As(err, &xerr) {
		return xerr.ExitCode
	}
	return -1
}

// Stderr returns the captured stderr from err, or "" if none.
func Stderr(err error) string {
	var xerr *Error
	if errors.As(err, &xerr) {
		return xerr.Stderr
	}
	return ""
}

// System is the real Runner backed by os/exec.
type System struct{}

func (System) Output(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		code := -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		}
		return "", &Error{
			Name:     name,
			Args:     args,
			Stderr:   strings.TrimSpace(stderr.String()),
			ExitCode: code,
			Err:      err,
		}
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

func (System) Interactive(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
