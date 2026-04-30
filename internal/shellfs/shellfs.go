// Package shellfs serves the ccs() shell-function templates that the
// `ccs-cli init` subcommand emits. Keeping them as standalone files (instead
// of inline string literals) lets developers edit + lint them as real shell
// scripts; the embed FS bakes them into the binary at build time.
package shellfs

import (
	"embed"
	"fmt"
)

//go:embed ccs.zsh ccs.bash ccs.fish
var fs embed.FS

// Script returns the shell integration script body for the named shell.
// Supported shells: "zsh", "bash", "fish". Any other value returns an error.
func Script(shell string) (string, error) {
	var name string
	switch shell {
	case "zsh":
		name = "ccs.zsh"
	case "bash":
		name = "ccs.bash"
	case "fish":
		name = "ccs.fish"
	default:
		return "", fmt.Errorf("unsupported shell: %s (use zsh, bash, or fish)", shell)
	}
	data, err := fs.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("read %s template: %w", name, err)
	}
	return string(data), nil
}
