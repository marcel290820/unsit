// Package notify shows desktop notifications through each OS's own system:
// freedesktop notifications over D-Bus on Linux, Notification Center on macOS
// and toasts on Windows.
package notify

import "context"

// Send shows a notification with a title and a short body, at normal urgency.
// Where the OS supports it, it replaces the previous one instead of piling up.
// It returns when ctx is done, even if the OS call is stuck past cancelling
// (godbus cannot cancel a connection attempt); that call is left behind.
func Send(ctx context.Context, title, body string) error {
	done := make(chan error, 1)
	go func() { done <- send(ctx, title, body) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
