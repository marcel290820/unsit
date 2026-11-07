package notify

import (
	"context"
	"fmt"
	"sync"

	"github.com/godbus/dbus/v5"
)

// The D-Bus connection is kept open between notifications. GNOME Shell ties
// a notification source to its sender's bus name when the sender matches an
// installed .desktop file (here "unsit.desktop", found via the app name), and
// destroys the source, with its notifications, as soon as that name vanishes.
// A connection closed right after each Notify made the banner disappear
// within milliseconds.
var (
	mu     sync.Mutex
	conn   *dbus.Conn
	lastID uint32 // lets each notification replace the previous one, so a desk left alone collects one reminder, not a stack of them
)

// connect opens the session bus connection. A private connection that never
// launches a bus: without a desktop session (say, over SSH) this fails instead
// of spawning a dbus-daemon. ctx only bounds the handshake, so that a stuck
// one ends; the connection itself outlives it.
func connect(ctx context.Context) (*dbus.Conn, error) {
	c, err := dbus.SessionBusPrivateNoAutoStartup()
	if err != nil {
		return nil, fmt.Errorf("no D-Bus session: %w", err)
	}
	stop := context.AfterFunc(ctx, func() { c.Close() })
	defer stop()
	if err := c.Auth(nil); err != nil {
		c.Close()
		return nil, fmt.Errorf("D-Bus auth: %w", err)
	}
	if err := c.Hello(); err != nil {
		c.Close()
		return nil, fmt.Errorf("D-Bus hello: %w", err)
	}
	return c, nil
}

func send(ctx context.Context, title, body string) error {
	mu.Lock()
	defer mu.Unlock()

	if conn == nil {
		c, err := connect(ctx)
		if err != nil {
			return err
		}
		conn = c
	}

	hints := map[string]dbus.Variant{"urgency": dbus.MakeVariant(byte(1))} // normal
	call := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications").CallWithContext(ctx,
		"org.freedesktop.Notifications.Notify", 0,
		"unsit", lastID, "", title, body, []string{}, hints, int32(-1)) // -1: server's default timeout
	var id uint32
	if err := call.Store(&id); err != nil {
		// Drop the connection so the next notification starts fresh.
		conn.Close()
		conn = nil
		return fmt.Errorf("freedesktop notification: %w", err)
	}
	lastID = id
	return nil
}
