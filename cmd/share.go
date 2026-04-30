package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aftership/ccs-cli/internal/output"
)

// sharedItems lists the entries we symlink from ~/.claude into a freshly
// added profile by default. Each entry is "extension definition",
// "installed software" or pure preference — nothing here can carry
// account credentials or session history. Account-bound state
// (.credentials.json, projects/, todos/, statsig/, settings.local.json,
// session/lock files, etc.) is intentionally absent and stays per-profile.
//
// Future Claude Code releases that introduce new credential or session
// files will stay isolated automatically because the list is a whitelist.
var sharedItems = []string{
	"CLAUDE.md",
	"agents",
	"commands",
	"skills",
	"output-styles",
	"keybindings.json",
	"hooks",
	"plugins",
	"settings.json",
}

// shareResult captures the outcome of one whitelist entry; it lets tests
// inspect the linker without parsing renderer output.
type shareResult struct {
	Item   string
	Status string // "linked" | "skipped" | "missing"
	Reason string // human-readable why, populated for skipped/missing
}

// linkSharedItems creates symlinks for each whitelist entry from sourceDir
// (typically the user's ~/.claude) into targetDir (the freshly created
// profile dir). Returns a slice of per-entry outcomes plus a fatal error
// for unexpected I/O failures (symlink errors). Conditions that are *not*
// errors:
//
//   - sourceDir does not exist (Claude Code never ran here yet)
//   - targetDir resolves to the same path as sourceDir (don't link to self)
//   - the named entry is missing from sourceDir
//   - the named entry already exists in targetDir
//
// All four return a non-fatal "skipped"/"missing" result so the caller can
// render a Skip line and move on.
func linkSharedItems(sourceDir, targetDir string) ([]shareResult, error) {
	sourceAbs, err := filepath.Abs(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("resolve source: %w", err)
	}
	targetAbs, err := filepath.Abs(targetDir)
	if err != nil {
		return nil, fmt.Errorf("resolve target: %w", err)
	}
	if sourceAbs == targetAbs {
		return []shareResult{{Status: "skipped", Reason: "target is the source dir"}}, nil
	}
	if _, err := os.Stat(sourceAbs); errors.Is(err, os.ErrNotExist) {
		return []shareResult{{Status: "skipped", Reason: "source dir does not exist: " + sourceAbs}}, nil
	} else if err != nil {
		return nil, fmt.Errorf("stat source: %w", err)
	}

	results := make([]shareResult, 0, len(sharedItems))
	for _, item := range sharedItems {
		results = append(results, linkOne(sourceAbs, targetAbs, item))
		if last := results[len(results)-1]; last.Status == "error" {
			return results, errors.New(last.Reason)
		}
	}
	return results, nil
}

func linkOne(sourceAbs, targetAbs, item string) shareResult {
	srcPath := filepath.Join(sourceAbs, item)
	dstPath := filepath.Join(targetAbs, item)

	if _, err := os.Lstat(srcPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return shareResult{Item: item, Status: "missing", Reason: "not present in source"}
		}
		return shareResult{Item: item, Status: "error", Reason: fmt.Sprintf("stat %s: %v", srcPath, err)}
	}

	if _, err := os.Lstat(dstPath); err == nil {
		return shareResult{Item: item, Status: "skipped", Reason: "already exists in target"}
	} else if !errors.Is(err, os.ErrNotExist) {
		return shareResult{Item: item, Status: "error", Reason: fmt.Sprintf("stat %s: %v", dstPath, err)}
	}

	if err := os.Symlink(srcPath, dstPath); err != nil {
		return shareResult{Item: item, Status: "error", Reason: fmt.Sprintf("symlink %s -> %s: %v", srcPath, dstPath, err)}
	}
	return shareResult{Item: item, Status: "linked"}
}

// renderShareResults emits per-item lines + a Summary tally. It's a thin
// adapter around the renderer so the linker stays pure.
//
// Items that are simply absent from the source (~/.claude doesn't have
// that subdir) are intentionally NOT rendered: the most common shape of
// a fresh ~/.claude is "only a couple of these exist", so showing seven
// "not present" lines on every add would be pure noise.
func renderShareResults(r *output.Renderer, results []shareResult) {
	for _, res := range results {
		switch res.Status {
		case "linked":
			r.Success("linked %s", r.Cyan(res.Item))
			r.Count("linked")
		case "skipped":
			if res.Item == "" {
				r.Skip("share skipped: %s", res.Reason)
				continue
			}
			r.Skip("%s (%s)", res.Item, res.Reason)
			r.Count("skipped")
		}
	}
	r.Summary("linked", "skipped")
}
