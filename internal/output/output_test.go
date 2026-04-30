package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestStatusLines_NoColor(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)

	r.Info("hello %s", "world")
	r.Success("ok")
	r.Warning("careful")
	r.Error("nope")
	r.Skip("later")

	got := buf.String()
	wants := []string{
		"  " + IconInfo + " hello world\n",
		"  " + IconSuccess + " ok\n",
		"  " + IconWarning + " careful\n",
		"  " + IconError + " nope\n",
		"  " + IconSkip + " later\n",
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("missing line %q in output:\n%s", w, got)
		}
	}
	if strings.Contains(got, "\033[") {
		t.Errorf("expected no ANSI escapes when color=false, got:\n%s", got)
	}
}

func TestStatusLines_WithColor(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, true)
	r.Success("done")

	got := buf.String()
	if !strings.Contains(got, "\033[32m") {
		t.Errorf("expected green ANSI escape, got %q", got)
	}
	if !strings.Contains(got, "\033[0m") {
		t.Errorf("expected reset ANSI escape, got %q", got)
	}
}

func TestSummary_OrderedAndOmitsZero(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)

	r.Count("added")
	r.Count("added")
	r.Count("skipped")
	r.Summary("added", "removed", "skipped")

	got := strings.TrimSpace(buf.String())
	want := "2 added, 1 skipped"
	if got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
}

func TestSummary_DefaultAlphabetical(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)

	r.Count("zebra")
	r.Count("apple")
	r.Summary()

	got := strings.TrimSpace(buf.String())
	want := "1 apple, 1 zebra"
	if got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
}

func TestTable_Alignment(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)

	r.Table(
		[]string{"NAME", "PATH"},
		[][]string{
			{"work", "/tmp/a"},
			{"personal-long", "/tmp/b"},
		},
	)

	got := buf.String()
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines (header + sep + 2 rows), got %d:\n%s", len(lines), got)
	}
	// Header column 1 padded to length of "personal-long"
	if !strings.HasPrefix(lines[0], "NAME         ") {
		t.Errorf("header not padded as expected: %q", lines[0])
	}
	if !strings.Contains(lines[1], "─") {
		t.Errorf("separator row missing: %q", lines[1])
	}
}

func TestTable_NoColor(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.Table([]string{"A"}, [][]string{{"1"}})
	if strings.Contains(buf.String(), "\033[") {
		t.Errorf("expected no ANSI in non-color table, got %q", buf.String())
	}
}

func TestPlain_NoNewline(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.Plain("export FOO=bar")
	if buf.String() != "export FOO=bar" {
		t.Errorf("Plain should not add newline, got %q", buf.String())
	}
}

func TestColorHelpers_WrapWhenColorEnabled(t *testing.T) {
	r := New(&bytes.Buffer{}, true)
	cases := []struct {
		name, escape string
		paint        func(string) string
	}{
		{"Bold", "\033[1m", r.Bold},
		{"Cyan", "\033[36m", r.Cyan},
		{"Green", "\033[32m", r.Green},
		{"Yellow", "\033[33m", r.Yellow},
		{"Red", "\033[31m", r.Red},
		{"Dim", "\033[2m", r.Dim},
	}
	for _, c := range cases {
		got := c.paint("hi")
		if !strings.HasPrefix(got, c.escape) || !strings.HasSuffix(got, "\033[0m") {
			t.Errorf("%s(%q) = %q, want prefix %q + reset suffix", c.name, "hi", got, c.escape)
		}
	}
}

func TestColorHelpers_PassthroughWhenColorDisabled(t *testing.T) {
	r := New(&bytes.Buffer{}, false)
	helpers := []func(string) string{r.Bold, r.Cyan, r.Green, r.Yellow, r.Red, r.Dim}
	for _, paint := range helpers {
		if got := paint("hi"); got != "hi" {
			t.Errorf("color disabled: got %q, want %q", got, "hi")
		}
	}
}

func TestVisibleWidth_StripsANSI(t *testing.T) {
	in := "\033[32mhello\033[0m"
	if got := visibleWidth(in); got != 5 {
		t.Errorf("visibleWidth(%q) = %d, want 5", in, got)
	}
}
