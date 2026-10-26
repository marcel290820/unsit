package notify

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func send(ctx context.Context, title, body string) error {
	// The text travels as arguments, never spliced into the script.
	out, err := exec.CommandContext(ctx, "/usr/bin/osascript",
		"-e", "on run argv",
		"-e", "display notification (item 2 of argv) with title (item 1 of argv)",
		"-e", "end run",
		title, body).CombinedOutput()
	if err != nil {
		return fmt.Errorf("osascript: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
