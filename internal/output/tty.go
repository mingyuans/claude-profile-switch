package output

import (
	"io"
	"os"
)

// IsTerminal reports whether w refers to an interactive terminal. The
// implementation deliberately avoids a dependency on golang.org/x/term so the
// module stays import-light; we only need stdout/stderr detection.
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
