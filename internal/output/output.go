// Package output renders Claude-style CLI output: status icons, indented
// permanent log lines, table rendering, and a final Summary aggregator.
//
// The package never reaches into business logic — callers feed strings and
// counters in, the renderer takes care of icon/colour/TTY handling.
package output

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
)

const (
	IconInfo    = "▸"
	IconSuccess = "✓"
	IconWarning = "!"
	IconError   = "✗"
	IconSkip    = "–"
)

// ANSI SGR codes used for permanent-line decoration. They are stripped when
// the writer is detected to be non-TTY (see Renderer.color).
const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
	ansiGray   = "\033[90m"
)

// Renderer is the single entry-point for all stdout writes. Callers should
// hold one Renderer per command invocation; Default() returns a process-wide
// instance backed by os.Stdout.
type Renderer struct {
	mu     sync.Mutex
	w      io.Writer
	color  bool
	counts map[string]int
}

// New returns a Renderer that writes to w. color controls ANSI escape
// emission; pass IsTerminal(w) to follow standard TTY auto-detect.
func New(w io.Writer, color bool) *Renderer {
	return &Renderer{
		w:      w,
		color:  color,
		counts: make(map[string]int),
	}
}

// Default returns a Renderer bound to os.Stdout with TTY-detected colour.
func Default() *Renderer {
	return New(os.Stdout, IsTerminal(os.Stdout))
}

// Header prints a bold section title surrounded by blank lines. Use sparingly
// — typically once at the start of a multi-step command.
func (r *Renderer) Header(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	r.write("\n" + r.paint(ansiBold, msg) + "\n\n")
}

// Info / Success / Warning / Error / Skip render a permanent log line with
// the canonical icon + colour from the be-guideline. Indent is two spaces to
// match the sample output in references/cli.md.
func (r *Renderer) Info(format string, args ...any) {
	r.line(IconInfo, ansiBold, format, args...)
}

func (r *Renderer) Success(format string, args ...any) {
	r.line(IconSuccess, ansiGreen, format, args...)
}

func (r *Renderer) Warning(format string, args ...any) {
	r.line(IconWarning, ansiYellow, format, args...)
}

func (r *Renderer) Error(format string, args ...any) {
	r.line(IconError, ansiRed, format, args...)
}

func (r *Renderer) Skip(format string, args ...any) {
	r.line(IconSkip, ansiGray, format, args...)
}

// Plain writes raw bytes without icon, indent or trailing newline. Use it for
// machine-consumable subcommands like `ccs init` or `ccs path`.
func (r *Renderer) Plain(format string, args ...any) {
	r.write(fmt.Sprintf(format, args...))
}

// Println writes a single line without any decoration.
func (r *Renderer) Println(format string, args ...any) {
	r.write(fmt.Sprintf(format, args...) + "\n")
}

// Count increments the named tally — called once per processed item. Summary
// flushes the tallies in the supplied order.
func (r *Renderer) Count(label string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counts[label]++
}

// Summary renders "N label, M label" using the order supplied. Labels with a
// zero count are omitted. If order is empty, labels are rendered alphabetically.
func (r *Renderer) Summary(order ...string) {
	r.mu.Lock()
	snapshot := make(map[string]int, len(r.counts))
	for k, v := range r.counts {
		snapshot[k] = v
	}
	r.mu.Unlock()

	keys := order
	if len(keys) == 0 {
		keys = make([]string, 0, len(snapshot))
		for k := range snapshot {
			keys = append(keys, k)
		}
		sort.Strings(keys)
	}

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		if n, ok := snapshot[k]; ok && n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, k))
		}
	}
	if len(parts) == 0 {
		return
	}
	r.write(strings.Join(parts, ", ") + "\n")
}

// Table renders a fixed-width text table. headers provides column titles;
// rows must have len(headers) cells each. Column widths grow to the longest
// rendered cell plus two trailing spaces.
func (r *Renderer) Table(headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}

	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = visibleWidth(h)
	}
	for _, row := range rows {
		for i := 0; i < len(headers) && i < len(row); i++ {
			if w := visibleWidth(row[i]); w > widths[i] {
				widths[i] = w
			}
		}
	}

	var b strings.Builder
	writeRow := func(cells []string, bold bool) {
		for i, w := range widths {
			cell := ""
			if i < len(cells) {
				cell = cells[i]
			}
			padded := cell + strings.Repeat(" ", w-visibleWidth(cell))
			if bold {
				padded = r.paint(ansiBold, padded)
			}
			b.WriteString(padded)
			if i < len(widths)-1 {
				b.WriteString("  ")
			}
		}
		b.WriteString("\n")
	}

	writeRow(headers, true)
	totalWidth := 0
	for _, w := range widths {
		totalWidth += w
	}
	totalWidth += 2 * (len(widths) - 1)
	b.WriteString(r.paint(ansiDim, strings.Repeat("─", totalWidth)))
	b.WriteString("\n")
	for _, row := range rows {
		writeRow(row, false)
	}
	r.write(b.String())
}

// Bold / Cyan / Green / Yellow / Red / Dim wrap s in the corresponding ANSI
// SGR sequence so callers can highlight key fragments inside formatted lines:
//
//	r.Success("switched to %s", r.Cyan(name))
//	r.Info("CLAUDE_CONFIG_DIR -> %s", r.Dim(dir))
//
// All helpers honour the renderer's TTY / colour setting — when colour is
// disabled they return s unchanged, so callers can wrap unconditionally.
func (r *Renderer) Bold(s string) string   { return r.paint(ansiBold, s) }
func (r *Renderer) Cyan(s string) string   { return r.paint(ansiCyan, s) }
func (r *Renderer) Green(s string) string  { return r.paint(ansiGreen, s) }
func (r *Renderer) Yellow(s string) string { return r.paint(ansiYellow, s) }
func (r *Renderer) Red(s string) string    { return r.paint(ansiRed, s) }
func (r *Renderer) Dim(s string) string    { return r.paint(ansiDim, s) }

// line is the private workhorse for Info/Success/etc.
func (r *Renderer) line(icon, color, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	r.write(fmt.Sprintf("  %s %s\n", r.paint(color, icon), msg))
}

func (r *Renderer) paint(color, s string) string {
	if !r.color || color == "" {
		return s
	}
	return color + s + ansiReset
}

func (r *Renderer) write(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = io.WriteString(r.w, s)
}

// visibleWidth returns the number of runes in s after stripping any ANSI SGR
// sequences. Sufficient for ASCII tables — wide-glyph alignment is out of
// scope for this CLI.
func visibleWidth(s string) int {
	n := 0
	inEscape := false
	for _, r := range s {
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		if r == '\033' {
			inEscape = true
			continue
		}
		n++
	}
	return n
}
