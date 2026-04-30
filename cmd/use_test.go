package cmd

import "testing"

func TestShellQuote(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/Users/foo/.claude-work", `'/Users/foo/.claude-work'`},
		{"/tmp/with space", `'/tmp/with space'`},
		{"/tmp/it's", `'/tmp/it'\''s'`},
		{"", `''`},
	}
	for _, c := range cases {
		got := shellQuote(c.in)
		if got != c.want {
			t.Errorf("shellQuote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
