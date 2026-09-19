package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// printer writes the compact ✓ ✘ ⚠ output shared by every command. Colour is
// used only when writing to a terminal and NO_COLOR is unset; the marks carry
// the meaning on their own.
type printer struct {
	w     io.Writer
	color bool
}

func newPrinter(w io.Writer) *printer {
	return &printer{w: w, color: isTerminal(w) && os.Getenv("NO_COLOR") == ""}
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

const (
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiBold   = "\x1b[1m"
	ansiReset  = "\x1b[0m"
)

func (p *printer) paint(code, s string) string {
	if !p.color {
		return s
	}
	return code + s + ansiReset
}

// line prints a plain line.
func (p *printer) line(format string, args ...any) {
	fmt.Fprintf(p.w, format+"\n", args...)
}

// blank prints an empty line.
func (p *printer) blank() { fmt.Fprintln(p.w) }

// title prints a bold heading.
func (p *printer) title(s string) { p.line("%s", p.paint(ansiBold, s)) }

// section prints a blank line followed by a heading.
func (p *printer) section(s string) {
	p.blank()
	p.line("%s", s)
}

// mark prints an indented status line: "  ✓ gopls".
func (p *printer) mark(code, mark, s string) {
	p.line("  %s %s", p.paint(code, mark), s)
}

func (p *printer) ok(s string)      { p.mark(ansiGreen, "✓", s) }
func (p *printer) missing(s string) { p.mark(ansiRed, "✘", s) }
func (p *printer) warn(s string)    { p.mark(ansiYellow, "⚠", s) }

// indented prints each line of a block with a two-space indent.
func (p *printer) indented(block string) {
	for _, l := range strings.Split(strings.TrimRight(block, "\n"), "\n") {
		p.line("  %s", l)
	}
}
