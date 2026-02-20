package tui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
)

// NotifyMsg is returned after a notification is delivered (no-op for the model).
type NotifyMsg struct{}

// notifyCmd returns a tea.Cmd that sends a desktop notification and rings the terminal bell.
func notifyCmd(sender, content string) tea.Cmd {
	return func() tea.Msg {
		// Terminal bell — write to stderr to bypass bubbletea's stdout
		os.Stderr.Write([]byte("\a"))

		// Desktop notification
		switch runtime.GOOS {
		case "darwin":
			title := fmt.Sprintf("tailchat — %s", sender)
			exec.Command("osascript", "-e",
				fmt.Sprintf(`display notification %q with title %q`, content, title),
			).Run()
		case "linux":
			exec.Command("notify-send",
				fmt.Sprintf("tailchat — %s", sender),
				content,
			).Run()
		}

		return NotifyMsg{}
	}
}
