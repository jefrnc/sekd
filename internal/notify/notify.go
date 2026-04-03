package notify

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Bell sends a terminal bell character.
func Bell() {
	fmt.Print("\a")
}

// Notify sends a system notification with title and message.
// Falls back to terminal bell if no notification system is available.
func Notify(title, message string) {
	switch runtime.GOOS {
	case "darwin":
		notifyMacOS(title, message)
	case "linux":
		notifyLinux(title, message)
	default:
		Bell()
	}
}

func notifyMacOS(title, message string) {
	// Check for iTerm2
	term := os.Getenv("TERM_PROGRAM")
	if strings.Contains(strings.ToLower(term), "iterm") {
		// iTerm2 proprietary escape sequence
		fmt.Printf("\033]9;%s\007", message)
		return
	}

	// Use osascript for native macOS notification
	script := fmt.Sprintf(`display notification "%s" with title "%s"`, message, title)
	exec.Command("osascript", "-e", script).Run()
}

func notifyLinux(title, message string) {
	if _, err := exec.LookPath("notify-send"); err == nil {
		exec.Command("notify-send", title, message).Run()
		return
	}
	Bell()
}
