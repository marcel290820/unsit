package tray

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"fyne.io/systray"
	"github.com/godbus/dbus/v5"

	"unsit/assets"
	"unsit/internal/timer"
)

// bus is the shared session bus connection that systray uses.
var bus *dbus.Conn

// setup waits briefly for a StatusNotifierItem host (Waybar, Omarchy's
// shell, KDE, ...), which systray itself would wait for silently forever.
func setup(ctx context.Context, first timer.State) error {
	// At login the bar may still be starting alongside unsit.
	probe, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := withBus(probe, func(conn *dbus.Conn) error {
		for !hostRegistered(probe, conn) {
			select {
			case <-probe.Done():
				return probe.Err()
			case <-time.After(250 * time.Millisecond):
			}
		}
		return nil
	})
	if err == nil {
		// systray connects with dbus.SessionBus, which can hang for good on a
		// stalled bus. Connect it here, in the probe's time; systray reuses it.
		shared := make(chan error, 1)
		go func() {
			conn, err := dbus.SessionBus()
			bus = conn
			shared <- err
		}()
		select {
		case err = <-shared:
		case <-probe.Done():
		}
	}
	switch {
	case ctx.Err() != nil:
		return ctx.Err()
	case probe.Err() != nil:
		return errors.New("no system tray found (no StatusNotifierItem host answered on D-Bus)")
	case err != nil:
		return err
	}

	// Hosts tell items apart by Id, which systray takes from the title set
	// before Run. A fixed Id lets a bar remember the icon across restarts
	// (Omarchy keeps pinned tray icons by Id).
	systray.SetTitle("unsit")
	// systray drops icon and tooltip changes made while it connects to D-Bus,
	// which overlaps the first Show. Set the first ones before it starts.
	text, _ := describe(first)
	systray.SetTooltip(text)
	setIcon(iconFor(first.Phase))
	return nil
}

// registered waits until the tray host lists unsit's icon. systray connects
// to D-Bus and registers the icon only after its ready callback, and just
// logs a failure to, so a host that refuses the icon would otherwise leave
// unsit running with no icon and no output.
func registered(ctx context.Context) bool {
	name := fmt.Sprintf("org.kde.StatusNotifierItem-%d-1", os.Getpid()) // systray's bus name
	err := withBus(ctx, func(conn *dbus.Conn) error {
		for !listed(ctx, conn, name) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(100 * time.Millisecond):
			}
		}
		return nil
	})
	return err == nil
}

// listed reports whether the StatusNotifierWatcher lists the icon behind
// name. Watchers list an icon as its bus name followed by its object path.
func listed(ctx context.Context, conn *dbus.Conn, name string) bool {
	var owner string
	if conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.GetNameOwner", 0, name).Store(&owner) != nil {
		return false // systray is not on the bus yet
	}
	var v dbus.Variant
	if err := watcher(conn).CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0,
		"org.kde.StatusNotifierWatcher", "RegisteredStatusNotifierItems").Store(&v); err != nil {
		return false // unconfirmed: asked again until Run gives up
	}
	items, _ := v.Value().([]string)
	for _, item := range items {
		if bus := itemBus(item); bus == owner || bus == name {
			return true
		}
	}
	return false
}

// itemBus returns the bus name of a RegisteredStatusNotifierItems entry:
// "bus/path" on Waybar and KDE, "bus@path" on GNOME's AppIndicator extension.
func itemBus(item string) string {
	bus, _, _ := strings.Cut(item, "/")
	bus, _, _ = strings.Cut(bus, "@")
	return bus
}

func started() {}

// abort quits systray, and closes its connection to end any D-Bus call it
// is stuck in: systray checks for quit only once its calls return.
func abort() {
	systray.Quit()
	bus.Close()
}

// withBus runs f on a new private session bus connection and returns when f
// does or ctx ends. The connection never launches a bus, so without a desktop
// session (say, over SSH) it fails, and it closes when ctx ends, which stops
// a stuck handshake or call. A stuck dial, which godbus can't cancel, is left
// behind.
func withBus(ctx context.Context, f func(*dbus.Conn) error) error {
	done := make(chan error, 1)
	go func() {
		conn, err := dbus.SessionBusPrivateNoAutoStartup(dbus.WithContext(ctx))
		if err != nil {
			done <- fmt.Errorf("no D-Bus session: %w", err)
			return
		}
		defer conn.Close()
		if err := conn.Auth(nil); err != nil {
			done <- fmt.Errorf("D-Bus auth: %w", err)
			return
		}
		if err := conn.Hello(); err != nil {
			done <- fmt.Errorf("D-Bus hello: %w", err)
			return
		}
		done <- f(conn)
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func hostRegistered(ctx context.Context, conn *dbus.Conn) bool {
	var v dbus.Variant
	err := watcher(conn).CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0,
		"org.kde.StatusNotifierWatcher", "IsStatusNotifierHostRegistered").Store(&v)
	registered, _ := v.Value().(bool)
	return err == nil && registered
}

func watcher(conn *dbus.Conn) dbus.BusObject {
	return conn.Object("org.kde.StatusNotifierWatcher", "/StatusNotifierWatcher")
}

func iconFor(p timer.Phase) []byte {
	light := lightTheme()
	switch {
	case p == timer.Standing && light:
		return assets.StandBlackPNG
	case p == timer.Standing:
		return assets.StandWhitePNG
	case light:
		return assets.SitBlackPNG
	default:
		return assets.SitWhitePNG
	}
}

func setIcon(icon []byte) { systray.SetIcon(icon) }

// lightTheme reports whether the desktop prefers a light look, the best hint
// at a light bar Linux offers. It asks the XDG settings portal, which
// Omarchy, GNOME and KDE keep current. Without an answer, it assumes dark.
func lightTheme() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var scheme uint32
	err := withBus(ctx, func(conn *dbus.Conn) error {
		var v dbus.Variant
		err := conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop").CallWithContext(ctx,
			"org.freedesktop.portal.Settings.ReadOne", 0,
			"org.freedesktop.appearance", "color-scheme").Store(&v)
		scheme, _ = v.Value().(uint32)
		return err
	})
	if err != nil {
		return false
	}
	return scheme == 2 // 0: no preference, 1: dark, 2: light
}
