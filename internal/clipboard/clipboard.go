package clipboard

import (
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripAnsi(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// Copy copies text to the system clipboard, stripping ANSI escape codes.
func Copy(text string) error {
	clean := stripAnsi(text)

	switch runtime.GOOS {
	case "darwin":
		return run("pbcopy", clean)
	case "linux":
		// Try xclip first, fall back to xsel
		if _, err := exec.LookPath("xclip"); err == nil {
			return run("xclip", clean, "-selection", "clipboard")
		}
		if _, err := exec.LookPath("xsel"); err == nil {
			return run("xsel", clean, "--clipboard", "--input")
		}
		return run("xclip", clean, "-selection", "clipboard")
	default:
		return run("clip", clean)
	}
}

func run(name string, input string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(input)
	return cmd.Run()
}
