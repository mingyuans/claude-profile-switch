package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// seedSourceItems creates a fake ~/.claude with the named entries. Files are
// touched as empty files; entries ending in "/" are created as directories.
// Returns the absolute source path.
func seedSourceItems(t *testing.T, items ...string) string {
	t.Helper()
	src := t.TempDir()
	for _, item := range items {
		full := filepath.Join(src, item)
		if filepath.Base(item) != item || hasSuffix(item, "/") {
			t.Fatalf("seedSourceItems: pass plain names like 'agents' or 'CLAUDE.md', got %q", item)
		}
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		if isDirEntry(item) {
			if err := os.MkdirAll(full, 0o755); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.WriteFile(full, []byte("seed"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return src
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

// isDirEntry treats anything without a "." in the basename as a directory
// (CLAUDE.md → file, agents → dir, keybindings.json → file).
func isDirEntry(name string) bool {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			return false
		}
	}
	return true
}

func TestLinkSharedItems_LinksWhitelistedEntriesPresentInSource(t *testing.T) {
	src := seedSourceItems(t, "CLAUDE.md", "agents", "skills")
	dst := t.TempDir()

	results, err := linkSharedItems(src, dst)
	if err != nil {
		t.Fatalf("linkSharedItems: %v", err)
	}

	wantLinked := map[string]bool{"CLAUDE.md": true, "agents": true, "skills": true}
	for _, res := range results {
		if !wantLinked[res.Item] {
			continue
		}
		if res.Status != "linked" {
			t.Errorf("%s status = %q, want linked", res.Item, res.Status)
		}
		dstPath := filepath.Join(dst, res.Item)
		target, err := os.Readlink(dstPath)
		if err != nil {
			t.Errorf("Readlink(%s): %v", dstPath, err)
			continue
		}
		if target != filepath.Join(src, res.Item) {
			t.Errorf("symlink target = %q, want %q", target, filepath.Join(src, res.Item))
		}
	}
}

func TestLinkSharedItems_SkipsEntriesAbsentFromSource(t *testing.T) {
	src := seedSourceItems(t, "agents") // no skills/, no CLAUDE.md
	dst := t.TempDir()

	results, err := linkSharedItems(src, dst)
	if err != nil {
		t.Fatal(err)
	}

	statusByItem := make(map[string]string)
	for _, res := range results {
		statusByItem[res.Item] = res.Status
	}
	if statusByItem["agents"] != "linked" {
		t.Errorf("agents status = %q, want linked", statusByItem["agents"])
	}
	if statusByItem["skills"] != "missing" {
		t.Errorf("skills status = %q, want missing", statusByItem["skills"])
	}
	if _, err := os.Lstat(filepath.Join(dst, "skills")); !os.IsNotExist(err) {
		t.Errorf("skills should not be created in target, got err = %v", err)
	}
}

func TestLinkSharedItems_DoesNotOverwriteExistingTargetEntries(t *testing.T) {
	src := seedSourceItems(t, "agents")
	dst := t.TempDir()

	// Pre-create a regular dir at the target — must not be replaced.
	preExisting := filepath.Join(dst, "agents")
	if err := os.MkdirAll(preExisting, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(preExisting, "do-not-touch.txt")
	if err := os.WriteFile(marker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := linkSharedItems(src, dst)
	if err != nil {
		t.Fatal(err)
	}

	for _, res := range results {
		if res.Item == "agents" {
			if res.Status != "skipped" {
				t.Errorf("agents status = %q, want skipped", res.Status)
			}
		}
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("pre-existing file got nuked: %v", err)
	}
	// Must still be a real dir, not a symlink.
	info, err := os.Lstat(preExisting)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("agents dir was replaced by a symlink")
	}
}

func TestLinkSharedItems_SourceMissingIsNotAnError(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "nope")
	dst := t.TempDir()

	results, err := linkSharedItems(src, dst)
	if err != nil {
		t.Fatalf("expected nil error for missing source, got %v", err)
	}
	if len(results) != 1 || results[0].Status != "skipped" {
		t.Errorf("expected single skipped result, got %+v", results)
	}
}

func TestLinkSharedItems_SelfLinkProtection(t *testing.T) {
	src := seedSourceItems(t, "agents")
	results, err := linkSharedItems(src, src)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != "skipped" {
		t.Errorf("expected single skipped self-link result, got %+v", results)
	}
	// agents/ in source must remain unchanged (not linked back to itself).
	info, err := os.Lstat(filepath.Join(src, "agents"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("source agents/ should not be a symlink after self-link guard")
	}
}
