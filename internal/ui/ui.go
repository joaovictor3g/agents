// Package ui renders all user-facing output: status symbols, colors, and
// tables. Color is disabled automatically when stdout is not a terminal,
// NO_COLOR is set, or TERM is dumb.
package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiDim    = "\x1b[2m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
)

// Printer writes styled output.
type Printer struct {
	out   io.Writer
	err   io.Writer
	color bool
}

// New returns a Printer for the process's stdout and stderr.
func New() *Printer {
	return &Printer{out: os.Stdout, err: os.Stderr, color: stdoutIsTerminal()}
}

// NewFor returns a Printer for arbitrary writers, used in tests.
func NewFor(out, err io.Writer, color bool) *Printer {
	return &Printer{out: out, err: err, color: color}
}

func stdoutIsTerminal() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	info, err := os.Stdout.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func (p *Printer) paint(code, s string) string {
	if !p.color {
		return s
	}
	return code + s + ansiReset
}

// Green paints s green when color is enabled.
func (p *Printer) Green(s string) string { return p.paint(ansiGreen, s) }

// Red paints s red when color is enabled.
func (p *Printer) Red(s string) string { return p.paint(ansiRed, s) }

// Yellow paints s yellow when color is enabled.
func (p *Printer) Yellow(s string) string { return p.paint(ansiYellow, s) }

// Dim paints s dim when color is enabled.
func (p *Printer) Dim(s string) string { return p.paint(ansiDim, s) }

// Bold paints s bold when color is enabled.
func (p *Printer) Bold(s string) string { return p.paint(ansiBold, s) }

// Success prints a ✔ line.
func (p *Printer) Success(format string, args ...any) {
	fmt.Fprintf(p.out, "%s %s\n", p.Green("✔"), fmt.Sprintf(format, args...))
}

// Info prints a plain line.
func (p *Printer) Info(format string, args ...any) {
	fmt.Fprintf(p.out, format+"\n", args...)
}

// Warn prints a ! line.
func (p *Printer) Warn(format string, args ...any) {
	fmt.Fprintf(p.out, "%s %s\n", p.Yellow("!"), fmt.Sprintf(format, args...))
}

// Error prints a ✘ line to stderr.
func (p *Printer) Error(err error) {
	fmt.Fprintf(p.err, "%s %s\n", p.Red("✘"), err)
}

// Table prints an aligned table with a header row.
func (p *Printer) Table(headers []string, rows [][]string) {
	tw := tabwriter.NewWriter(p.out, 0, 4, 3, ' ', 0)
	fmt.Fprintln(tw, p.Bold(strings.Join(headers, "\t")))
	for _, row := range rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	tw.Flush()
}
