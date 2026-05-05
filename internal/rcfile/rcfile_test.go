package rcfile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPath(t *testing.T) {
	tmp := t.TempDir()

	t.Run("zsh default", func(t *testing.T) {
		t.Setenv("ZDOTDIR", "")
		got, err := Path("zsh", tmp)
		if err != nil {
			t.Fatal(err)
		}
		if got != filepath.Join(tmp, ".zshrc") {
			t.Errorf("got %q", got)
		}
	})

	t.Run("zsh ZDOTDIR override", func(t *testing.T) {
		zdot := t.TempDir()
		t.Setenv("ZDOTDIR", zdot)
		got, err := Path("zsh", tmp)
		if err != nil {
			t.Fatal(err)
		}
		if got != filepath.Join(zdot, ".zshrc") {
			t.Errorf("got %q want %q", got, filepath.Join(zdot, ".zshrc"))
		}
	})

	t.Run("bash prefers .bash_profile when present", func(t *testing.T) {
		home := t.TempDir()
		profile := filepath.Join(home, ".bash_profile")
		if err := os.WriteFile(profile, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := Path("bash", home)
		if err != nil {
			t.Fatal(err)
		}
		if got != profile {
			t.Errorf("got %q want %q", got, profile)
		}
	})

	t.Run("bash falls back to .bashrc when no .bash_profile", func(t *testing.T) {
		home := t.TempDir()
		got, err := Path("bash", home)
		if err != nil {
			t.Fatal(err)
		}
		if got != filepath.Join(home, ".bashrc") {
			t.Errorf("got %q", got)
		}
	})

	t.Run("fish default", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		got, err := Path("fish", tmp)
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(tmp, ".config", "fish", "config.fish")
		if got != want {
			t.Errorf("got %q want %q", got, want)
		}
	})

	t.Run("fish XDG_CONFIG_HOME override", func(t *testing.T) {
		xdg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", xdg)
		got, err := Path("fish", tmp)
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(xdg, "fish", "config.fish")
		if got != want {
			t.Errorf("got %q want %q", got, want)
		}
	})

	t.Run("unsupported shell", func(t *testing.T) {
		_, err := Path("dash", tmp)
		if !errors.Is(err, ErrUnsupportedShell) {
			t.Errorf("got %v, want ErrUnsupportedShell", err)
		}
	})
}

func TestDetectShell(t *testing.T) {
	cases := []struct {
		shellEnv string
		want     string
		wantErr  bool
	}{
		{"/bin/zsh", "zsh", false},
		{"/usr/local/bin/bash", "bash", false},
		{"/opt/homebrew/bin/fish", "fish", false},
		{"zsh", "zsh", false},
		{"", "", true},
		{"/bin/dash", "", true},
	}
	for _, c := range cases {
		got, err := DetectShell(c.shellEnv)
		if (err != nil) != c.wantErr {
			t.Errorf("DetectShell(%q): err=%v, wantErr=%v", c.shellEnv, err, c.wantErr)
			continue
		}
		if got != c.want {
			t.Errorf("DetectShell(%q) = %q, want %q", c.shellEnv, got, c.want)
		}
	}
}

func TestExportLine(t *testing.T) {
	cases := []struct {
		shell, dir, want string
		wantErr          bool
	}{
		{"zsh", "/Users/xq.yan/.claude-xq.yan", `export CLAUDE_CONFIG_DIR='/Users/xq.yan/.claude-xq.yan'`, false},
		{"bash", "/Users/xq.yan/.claude-xq.yan", `export CLAUDE_CONFIG_DIR='/Users/xq.yan/.claude-xq.yan'`, false},
		{"zsh", "/tmp/it's", `export CLAUDE_CONFIG_DIR='/tmp/it'\''s'`, false},
		{"fish", "/p", `set -gx CLAUDE_CONFIG_DIR '/p'`, false},
		{"fish", "/tmp/it's", `set -gx CLAUDE_CONFIG_DIR '/tmp/it\'s'`, false},
		{"dash", "/p", "", true},
	}
	for _, c := range cases {
		got, err := ExportLine(c.shell, c.dir)
		if (err != nil) != c.wantErr {
			t.Errorf("ExportLine(%q,%q): err=%v wantErr=%v", c.shell, c.dir, err, c.wantErr)
			continue
		}
		if got != c.want {
			t.Errorf("ExportLine(%q,%q) = %q, want %q", c.shell, c.dir, got, c.want)
		}
	}
}

func TestUpdate_CreatesFileWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".zshrc")

	if err := Update(path, "export CLAUDE_CONFIG_DIR='/p'"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, BeginMarker) || !strings.Contains(got, EndMarker) {
		t.Errorf("missing markers:\n%s", got)
	}
	if !strings.Contains(got, "export CLAUDE_CONFIG_DIR='/p'") {
		t.Errorf("missing export line:\n%s", got)
	}
}

func TestUpdate_AppendsBlockToExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".zshrc")
	original := "# user content\nexport PATH=$HOME/bin:$PATH\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Update(path, "export CLAUDE_CONFIG_DIR='/p'"); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	got := string(data)
	if !strings.HasPrefix(got, original) {
		t.Errorf("original content not preserved at top:\n%s", got)
	}
	if !strings.Contains(got, BeginMarker) || !strings.Contains(got, EndMarker) {
		t.Errorf("missing markers:\n%s", got)
	}
	if !strings.Contains(got, "export CLAUDE_CONFIG_DIR='/p'") {
		t.Errorf("missing export line:\n%s", got)
	}
}

func TestUpdate_ReplacesExistingBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".zshrc")
	original := "# top\n" + BeginMarker + "\nexport CLAUDE_CONFIG_DIR='/old'\n" + EndMarker + "\n# bottom\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Update(path, "export CLAUDE_CONFIG_DIR='/new'"); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	got := string(data)
	if strings.Contains(got, "/old") {
		t.Errorf("old line still present:\n%s", got)
	}
	if !strings.Contains(got, "/new") {
		t.Errorf("new line missing:\n%s", got)
	}
	if !strings.Contains(got, "# top") || !strings.Contains(got, "# bottom") {
		t.Errorf("surrounding content lost:\n%s", got)
	}
	if strings.Count(got, BeginMarker) != 1 || strings.Count(got, EndMarker) != 1 {
		t.Errorf("expected exactly one marker block, got:\n%s", got)
	}
}

func TestUpdate_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".zshrc")

	if err := Update(path, "export CLAUDE_CONFIG_DIR='/p'"); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(path)

	if err := Update(path, "export CLAUDE_CONFIG_DIR='/p'"); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(path)

	if string(first) != string(second) {
		t.Errorf("not idempotent.\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	if strings.Count(string(second), BeginMarker) != 1 {
		t.Errorf("multiple blocks after second Update:\n%s", second)
	}
}

func TestUpdate_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "config.fish")

	if err := Update(path, "set -gx CLAUDE_CONFIG_DIR '/p'"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

func TestReadExportedDir_MissingFile(t *testing.T) {
	dir := t.TempDir()
	got, ok, err := ReadExportedDir(filepath.Join(dir, "no-such-file"))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if ok || got != "" {
		t.Errorf("got (%q, %v), want (\"\", false)", got, ok)
	}
}

func TestReadExportedDir_NoBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".zshrc")
	if err := os.WriteFile(path, []byte("# user content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok, err := ReadExportedDir(path)
	if err != nil {
		t.Fatal(err)
	}
	if ok || got != "" {
		t.Errorf("got (%q, %v), want (\"\", false)", got, ok)
	}
}

func TestReadExportedDir_Roundtrip(t *testing.T) {
	cases := []struct {
		name, shell, dir string
	}{
		{"zsh simple", "zsh", "/Users/foo/.claude-work"},
		{"bash with space", "bash", "/tmp/with space"},
		{"zsh with quote", "zsh", "/tmp/it's"},
		{"fish simple", "fish", "/Users/foo/.claude-work"},
		{"fish with quote", "fish", "/tmp/it's"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tmp := t.TempDir()
			path := filepath.Join(tmp, "rcfile")
			line, err := ExportLine(c.shell, c.dir)
			if err != nil {
				t.Fatal(err)
			}
			if err := Update(path, line); err != nil {
				t.Fatal(err)
			}

			got, ok, err := ReadExportedDir(path)
			if err != nil {
				t.Fatal(err)
			}
			if !ok {
				t.Fatalf("ok = false; expected to find a dir")
			}
			if got != c.dir {
				t.Errorf("got %q, want %q", got, c.dir)
			}
		})
	}
}

func TestReadExportedDir_IgnoresSurroundingContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".zshrc")
	body := "# pre\nexport CLAUDE_CONFIG_DIR='/wrong/place'\n" +
		BeginMarker + "\nexport CLAUDE_CONFIG_DIR='/right/place'\n" + EndMarker + "\n" +
		"# post\nexport CLAUDE_CONFIG_DIR='/also/wrong'\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok, err := ReadExportedDir(path)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("ok = false")
	}
	if got != "/right/place" {
		t.Errorf("got %q, want /right/place — only the managed block should be parsed", got)
	}
}
