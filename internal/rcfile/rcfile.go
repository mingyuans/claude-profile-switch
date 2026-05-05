// Package rcfile maintains a managed export block in the user's shell rc
// file (e.g. ~/.zshrc) so that CLAUDE_CONFIG_DIR persists across new shell
// sessions. The block is delimited by BeginMarker / EndMarker; content
// outside the markers is never touched.
package rcfile

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrUnsupportedShell is returned by Path / ExportLine / DetectShell when
// the shell is not one of zsh, bash, fish.
var ErrUnsupportedShell = errors.New("unsupported shell")

// BeginMarker / EndMarker delimit the rc-file region this package owns.
// The strings are documented in the rendered banner so users who open
// their rc file know which tool to blame for changes inside.
const (
	BeginMarker = "# >>> ccs-cli >>> managed block; updated by `ccs use`"
	EndMarker   = "# <<< ccs-cli <<< end managed block"
)

// Path resolves the rc file path for shell, honouring ZDOTDIR (zsh) and
// XDG_CONFIG_HOME (fish). For bash it prefers ~/.bash_profile when that
// file already exists (the macOS login-shell convention) and falls back
// to ~/.bashrc otherwise.
func Path(shell, home string) (string, error) {
	switch shell {
	case "zsh":
		if zdot := os.Getenv("ZDOTDIR"); zdot != "" {
			return filepath.Join(zdot, ".zshrc"), nil
		}
		return filepath.Join(home, ".zshrc"), nil
	case "bash":
		profile := filepath.Join(home, ".bash_profile")
		if _, err := os.Stat(profile); err == nil {
			return profile, nil
		}
		return filepath.Join(home, ".bashrc"), nil
	case "fish":
		base := os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			base = filepath.Join(home, ".config")
		}
		return filepath.Join(base, "fish", "config.fish"), nil
	default:
		return "", fmt.Errorf("%w: %q (use zsh, bash, or fish)", ErrUnsupportedShell, shell)
	}
}

// DetectShell returns the canonical shell name for a $SHELL value such
// as "/bin/zsh". Returns ErrUnsupportedShell when shellEnv is empty or
// not a supported shell.
func DetectShell(shellEnv string) (string, error) {
	name := filepath.Base(strings.TrimSpace(shellEnv))
	switch name {
	case "zsh", "bash", "fish":
		return name, nil
	case "", ".":
		return "", fmt.Errorf("%w: $SHELL is empty", ErrUnsupportedShell)
	default:
		return "", fmt.Errorf("%w: %q", ErrUnsupportedShell, name)
	}
}

// ExportLine returns the shell-syntax line that exports
// CLAUDE_CONFIG_DIR=dir for shell. The dir is single-quote escaped so
// embedded quotes / spaces are safe.
func ExportLine(shell, dir string) (string, error) {
	switch shell {
	case "zsh", "bash":
		return "export CLAUDE_CONFIG_DIR=" + posixSingleQuote(dir), nil
	case "fish":
		return "set -gx CLAUDE_CONFIG_DIR " + fishSingleQuote(dir), nil
	default:
		return "", fmt.Errorf("%w: %q", ErrUnsupportedShell, shell)
	}
}

// Update rewrites the managed block in path so it contains exactly line.
// Behaviour:
//
//   - Missing file: parent dirs are created (0o755) and a new file is
//     written containing only the block.
//   - File exists without markers: the block is appended at the end,
//     separated by a blank line from prior content.
//   - File exists with markers: the body between markers is replaced;
//     content outside is preserved byte-for-byte.
//
// Writes are atomic via tmp file + rename so a crash leaves the previous
// rc file intact.
func Update(path, line string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create rc dir: %w", err)
	}

	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read %s: %w", path, err)
	}

	updated := replaceOrAppendBlock(existing, line)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, updated, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}

// replaceOrAppendBlock returns the new file body with the managed block
// containing exactly `line` between BeginMarker and EndMarker.
func replaceOrAppendBlock(existing []byte, line string) []byte {
	block := BeginMarker + "\n" + line + "\n" + EndMarker + "\n"

	if len(existing) == 0 {
		return []byte(block)
	}

	beginIdx, endIdx, ok := findBlock(existing)
	if !ok {
		var buf bytes.Buffer
		buf.Write(existing)
		if !bytes.HasSuffix(existing, []byte("\n")) {
			buf.WriteByte('\n')
		}
		buf.WriteByte('\n')
		buf.WriteString(block)
		return buf.Bytes()
	}

	var buf bytes.Buffer
	buf.Write(existing[:beginIdx])
	buf.WriteString(block)
	buf.Write(existing[endIdx:])
	return buf.Bytes()
}

// findBlock locates the managed block by scanning lines. Returns the
// byte offsets [begin, end) suitable for splice + the lines after the
// EndMarker line (including its terminating newline).
func findBlock(data []byte) (begin, end int, ok bool) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	pos := 0
	beginLineStart := -1
	for scanner.Scan() {
		line := scanner.Text()
		lineLen := len(line) + 1 // assume \n; tolerate missing trailing newline below
		if line == BeginMarker && beginLineStart == -1 {
			beginLineStart = pos
		} else if line == EndMarker && beginLineStart != -1 {
			return beginLineStart, min(pos+lineLen, len(data)), true
		}
		pos = min(pos+lineLen, len(data))
	}
	return 0, 0, false
}

// ReadExportedDir extracts the CLAUDE_CONFIG_DIR value from the managed
// block of path. Returns ("", false, nil) when the file or block is
// missing — these are normal "no profile persisted yet" states, not
// errors. A real I/O failure (other than ENOENT) is surfaced.
//
// Recognises both POSIX-style (`export CLAUDE_CONFIG_DIR='/dir'`) and
// fish-style (`set -gx CLAUDE_CONFIG_DIR '/dir'`) lines, and unescapes
// the corresponding quote conventions so callers get the literal path.
func ReadExportedDir(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read %s: %w", path, err)
	}
	begin, end, ok := findBlock(data)
	if !ok {
		return "", false, nil
	}
	body := string(data[begin:end])
	for line := range strings.SplitSeq(body, "\n") {
		if dir, ok := parseExportLine(line); ok {
			return dir, true, nil
		}
	}
	return "", false, nil
}

// parseExportLine recognises the line shapes ExportLine produces and
// returns the unquoted dir. Returns ("", false) for marker / blank /
// unrelated lines.
func parseExportLine(line string) (string, bool) {
	const (
		posixPrefix = "export CLAUDE_CONFIG_DIR="
		fishPrefix  = "set -gx CLAUDE_CONFIG_DIR "
	)
	switch {
	case strings.HasPrefix(line, posixPrefix):
		return posixUnquote(strings.TrimPrefix(line, posixPrefix))
	case strings.HasPrefix(line, fishPrefix):
		return fishUnquote(strings.TrimPrefix(line, fishPrefix))
	}
	return "", false
}

// posixUnquote inverts posixSingleQuote: strips the outer single quotes
// and turns each '\'' sequence back into a single quote.
func posixUnquote(s string) (string, bool) {
	if len(s) < 2 || s[0] != '\'' || s[len(s)-1] != '\'' {
		return "", false
	}
	inner := s[1 : len(s)-1]
	return strings.ReplaceAll(inner, `'\''`, "'"), true
}

// fishUnquote inverts fishSingleQuote: strips outer single quotes and
// turns \' back into a single quote.
func fishUnquote(s string) (string, bool) {
	if len(s) < 2 || s[0] != '\'' || s[len(s)-1] != '\'' {
		return "", false
	}
	inner := s[1 : len(s)-1]
	return strings.ReplaceAll(inner, `\'`, "'"), true
}

// posixSingleQuote wraps s in single quotes, escaping embedded single
// quotes via the standard '\'' trick. Safe for sh / bash / zsh.
func posixSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// fishSingleQuote wraps s in single quotes for fish, where embedded
// single quotes are escaped as \' (no '\” trick).
func fishSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `\'`) + "'"
}
