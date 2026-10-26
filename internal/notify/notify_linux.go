package notify

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/godbus/dbus/v5"
)

// lastID lets each notification replace the previous one, so a desk left
// alone collects one reminder, not a stack of them.
var lastID atomic.Uint32

func send(ctx context.Context, title, body string) error {
	// A private connection that never launches a bus: without a desktop
	// session (say, over SSH) this fails instead of spawning a dbus-daemon.
	// It closes when ctx is done, which also ends a stuck handshake.
	conn, err := dbus.SessionBusPrivateNoAutoStartup(dbus.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("no D-Bus session: %w", err)
	}
	defer conn.Close()
	if err := conn.Auth(nil); err != nil {
		return fmt.Errorf("D-Bus auth: %w", err)
	}
	if err := conn.Hello(); err != nil {
		return fmt.Errorf("D-Bus hello: %w", err)
	}

	hints := map[string]dbus.Variant{"urgency": dbus.MakeVariant(byte(1))} // normal
	call := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications").CallWithContext(ctx,
		"org.freedesktop.Notifications.Notify", 0,
		"unsit", lastID.Load(), "", title, body, []string{}, hints, int32(-1)) // -1: server's default timeout
	var id uint32
	if err := call.Store(&id); err != nil {
		return fmt.Errorf("freedesktop notification: %w", err)
	}
	lastID.Store(id)
	return nil
}
