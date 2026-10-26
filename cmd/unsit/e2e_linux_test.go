package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

// The end-to-end tests build unsit and run it against a fake desktop on a
// private session bus: a notification server, the XDG settings portal and,
// optionally, a StatusNotifierItem host.

type notification struct {
	summary, body string
	replaces, id  uint32
	urgency       byte
	timeout       int32
}

type desktop struct {
	t     *testing.T
	bus   string // the session bus socket
	conn  *dbus.Conn
	mu    sync.Mutex
	notes []notification
	scene uint32           // color-scheme: 1 dark, 2 light
	item  string           // bus name of the registered tray item
	stuck chan struct{}    // when set, Notify records the call, then hangs until it closes
	tray  *prop.Properties // the tray host's properties; nil when it keeps no icon list
	deny  bool             // the tray host refuses to show icons
}

func (d *desktop) Notify(app string, replaces uint32, icon, summary, body string, actions []string, hints map[string]dbus.Variant, timeout int32) (uint32, *dbus.Error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	urgency, _ := hints["urgency"].Value().(byte)
	id := uint32(len(d.notes) + 1)
	if replaces != 0 {
		id = replaces
	}
	d.notes = append(d.notes, notification{summary, body, replaces, id, urgency, timeout})
	if d.stuck != nil {
		d.mu.Unlock()
		<-d.stuck
		d.mu.Lock()
	}
	return id, nil
}

func (d *desktop) ReadOne(namespace, key string) (dbus.Variant, *dbus.Error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return dbus.MakeVariant(d.scene), nil
}

func (d *desktop) RegisterStatusNotifierItem(sender dbus.Sender, path string) *dbus.Error {
	if d.deny {
		return dbus.MakeFailedError(errors.New("no room for icons"))
	}
	d.mu.Lock()
	d.item = string(sender)
	d.mu.Unlock()
	if d.tray == nil {
		return nil
	}
	d.tray.SetMust("org.kde.StatusNotifierWatcher", "RegisteredStatusNotifierItems", []string{string(sender) + path})
	return nil
}

// hungWatcher owns the StatusNotifierWatcher name but never answers.
type hungWatcher chan struct{}

func (h hungWatcher) Get(iface, property string) (dbus.Variant, *dbus.Error) {
	<-h
	return dbus.Variant{}, nil
}

type trayHost int

const (
	noHost trayHost = iota
	workingHost
	hungHost
	denyingHost
	listlessHost // shows icons but doesn't list them
)

func newDesktop(t *testing.T, host trayHost) *desktop {
	daemon, err := exec.LookPath("dbus-daemon")
	if err != nil {
		t.Skip("needs dbus-daemon")
	}
	sock := filepath.Join(t.TempDir(), "bus")
	bus := exec.Command(daemon, "--session", "--nofork", "--address=unix:path="+sock, "--print-address=1")
	out, err := bus.StdoutPipe()
	must(t, err)
	must(t, bus.Start())
	t.Cleanup(func() { bus.Process.Kill(); bus.Wait() })
	addr, err := bufio.NewReader(out).ReadString('\n')
	must(t, err)
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", strings.TrimSpace(addr))

	conn, err := dbus.ConnectSessionBus()
	must(t, err)
	t.Cleanup(func() { conn.Close() })
	d := &desktop{t: t, bus: sock, conn: conn, scene: 1}
	serve := func(name, path, iface string) {
		must(t, conn.Export(d, dbus.ObjectPath(path), iface))
		own(t, conn, name)
	}
	serve("org.freedesktop.Notifications", "/org/freedesktop/Notifications", "org.freedesktop.Notifications")
	serve("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop", "org.freedesktop.portal.Settings")
	switch host {
	case workingHost, denyingHost, listlessHost:
		d.deny = host == denyingHost
		props := map[string]*prop.Prop{"IsStatusNotifierHostRegistered": {Value: true}}
		if host != listlessHost {
			props["RegisteredStatusNotifierItems"] = &prop.Prop{Value: []string{}, Emit: prop.EmitTrue}
		}
		tray, err := prop.Export(conn, "/StatusNotifierWatcher", prop.Map{"org.kde.StatusNotifierWatcher": props})
		must(t, err)
		if host != listlessHost {
			d.tray = tray
		}
		serve("org.kde.StatusNotifierWatcher", "/StatusNotifierWatcher", "org.kde.StatusNotifierWatcher")
	case hungHost:
		hung := make(hungWatcher)
		t.Cleanup(func() { close(hung) })
		must(t, conn.Export(hung, "/StatusNotifierWatcher", "org.freedesktop.DBus.Properties"))
		own(t, conn, "org.kde.StatusNotifierWatcher")
	}
	return d
}

func own(t *testing.T, conn *dbus.Conn, name string) {
	t.Helper()
	if reply, err := conn.RequestName(name, dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("own %s: %v", name, err)
	}
}

// eventually polls cond for up to timeout.
func eventually(t *testing.T, what string, timeout time.Duration, cond func() bool) {
	t.Helper()
	for end := time.Now().Add(timeout); !cond(); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(end) {
			t.Fatalf("timed out waiting for %s", what)
		}
	}
}

func (d *desktop) notifications() []notification {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]notification(nil), d.notes...)
}

func (d *desktop) setScheme(scheme uint32) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.scene = scheme
}

func (d *desktop) itemName() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.item
}

func (d *desktop) itemProp(name string) any {
	d.t.Helper()
	v, err := d.conn.Object(d.itemName(), "/StatusNotifierItem").GetProperty("org.kde.StatusNotifierItem." + name)
	must(d.t, err)
	return v.Value()
}

// iconColor is the color of the tray icon's opaque pixels.
func (d *desktop) iconColor() string {
	px := d.itemProp("IconPixmap").([][]any)
	argb := px[0][2].([]byte)
	for i := 0; i < len(argb); i += 4 {
		if argb[i] == 255 {
			return map[byte]string{0: "black", 255: "white"}[argb[i+1]]
		}
	}
	return "none"
}

func (d *desktop) tooltip() string { return d.itemProp("ToolTip").([]any)[2].(string) }

// menu returns the tray menu's labels, with the ID of Stop.
func (d *desktop) menu() (labels []string, stopID int32) {
	var layout struct {
		ID       int32
		Props    map[string]dbus.Variant
		Children []dbus.Variant
	}
	var revision uint32
	must(d.t, d.conn.Object(d.itemName(), "/StatusNotifierItem/menu").Call(
		"com.canonical.dbusmenu.GetLayout", 0, int32(0), int32(-1), []string{}).Store(&revision, &layout))
	for _, child := range layout.Children {
		fields := child.Value().([]any)
		props := fields[1].(map[string]dbus.Variant)
		label, _ := props["label"].Value().(string)
		if kind, _ := props["type"].Value().(string); kind == "separator" {
			label = "-"
		}
		if enabled, ok := props["enabled"].Value().(bool); ok && !enabled {
			label += " (disabled)"
		}
		if label == "Stop" {
			stopID = fields[0].(int32)
		}
		labels = append(labels, label)
	}
	return labels, stopID
}

// startUnsit builds and starts unsit. A separate build, not this test binary:
// under -race it would also report races inside fyne.io/systray.
func startUnsit(t *testing.T, args ...string) (*exec.Cmd, *strings.Builder, *strings.Builder, chan error) {
	bin := filepath.Join(t.TempDir(), "unsit")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	cmd := exec.Command(bin, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	must(t, cmd.Start())
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	t.Cleanup(func() { cmd.Process.Kill() })
	return cmd, &stdout, &stderr, exited
}

func waitExit(t *testing.T, exited chan error, stderr *strings.Builder) {
	t.Helper()
	select {
	case err := <-exited:
		if err != nil {
			t.Fatalf("unsit exited with %v; stderr: %s", err, stderr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("unsit did not exit")
	}
}

func TestEndToEndTray(t *testing.T) {
	if testing.Short() {
		t.Skip("runs a real 2s/1s cycle")
	}
	d := newDesktop(t, workingHost)
	_, _, stderr, exited := startUnsit(t, "--sit", "2s", "--stand", "1s")

	eventually(t, "the tray item", 5*time.Second, func() bool { return d.itemName() != "" })
	if id := d.itemProp("Id"); id != "unsit" {
		t.Errorf("item Id = %v, want unsit (a stable Id lets bars remember the icon)", id)
	}
	if got := d.tooltip(); got != "Sitting · 1 min left" {
		t.Errorf("tooltip = %q", got)
	}
	if got := d.iconColor(); got != "white" {
		t.Errorf("icon on a dark desktop = %s, want white", got)
	}
	labels, stopID := d.menu()
	if want := []string{"Sitting · 1 min left (disabled)", "-", "Stop"}; strings.Join(labels, "|") != strings.Join(want, "|") {
		t.Errorf("menu = %q, want %q", labels, want)
	}

	eventually(t, "standing", 4*time.Second, func() bool { return d.tooltip() == "Standing · 1 min left" })
	d.setScheme(2) // the desktop switches to a light theme
	eventually(t, "sitting again, with a black icon", 4*time.Second, func() bool {
		return d.tooltip() == "Sitting · 1 min left" && d.iconColor() == "black"
	})
	want := []notification{
		{summary: "Stand up", body: "1 second standing", replaces: 0, id: 1, urgency: 1, timeout: -1},
		{summary: "Sit down", body: "2 seconds sitting", replaces: 1, id: 1, urgency: 1, timeout: -1},
	}
	if got := d.notifications(); len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("notifications = %+v, want %+v", got, want)
	}

	must(t, d.conn.Object(d.itemName(), "/StatusNotifierItem/menu").Call(
		"com.canonical.dbusmenu.Event", 0, stopID, "clicked", dbus.MakeVariant(""), uint32(0)).Err)
	waitExit(t, exited, stderr)
	time.Sleep(2500 * time.Millisecond) // past the next switch
	if n := len(d.notifications()); n != 2 {
		t.Errorf("got %d notifications after Stop", n-2)
	}
}

func TestEndToEndNoTrayFallback(t *testing.T) {
	if testing.Short() {
		t.Skip("waits out the 5s tray grace period")
	}
	for name, tc := range map[string]struct {
		host    trayHost
		warning string
	}{
		"no tray host":             {noHost, "no system tray found"},
		"tray host never answers":  {hungHost, "no system tray found"},
		"tray host refuses icons":  {denyingHost, "the tray icon did not start"},
		"tray host lists no icons": {listlessHost, "the tray icon did not start"},
	} {
		t.Run(name, func(t *testing.T) {
			d := newDesktop(t, tc.host)
			cmd, stdout, stderr, exited := startUnsit(t, "--sit", "1s", "--stand", "1s")

			eventually(t, "a notification", 9*time.Second, func() bool { return len(d.notifications()) == 1 })
			must(t, cmd.Process.Signal(syscall.SIGINT))
			waitExit(t, exited, stderr)
			// systray may log its own failure first.
			if !strings.Contains(stderr.String(), tc.warning) || strings.Count(stderr.String(), "unsit: ") != 1 {
				t.Errorf("stderr = %q, want one no-tray warning", stderr)
			}
			lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
			if len(lines) != 2 || !strings.HasSuffix(lines[0], "Sitting for 1 second. Stop with Ctrl-C.") ||
				!strings.HasSuffix(lines[1], "Stand up: 1 second standing") {
				t.Errorf("stdout = %q", stdout)
			}
		})
	}
}

// A notification server that hangs must not keep Ctrl-C from stopping unsit.
func TestEndToEndStopDuringStuckNotification(t *testing.T) {
	if testing.Short() {
		t.Skip("runs a real 1s phase")
	}
	d := newDesktop(t, noHost)
	d.stuck = make(chan struct{})
	t.Cleanup(func() { close(d.stuck) })
	cmd, _, stderr, exited := startUnsit(t, "--no-tray", "--sit", "1s", "--stand", "1s")

	eventually(t, "the notification call", 5*time.Second, func() bool { return len(d.notifications()) == 1 })
	must(t, cmd.Process.Signal(syscall.SIGINT))
	waitExit(t, exited, stderr)
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want nothing: stopping is not a failure", stderr)
	}
}

// A session bus that stops answering, from the start or later on, must not
// hang startup, the cycle, or stopping.
func TestEndToEndStuckBus(t *testing.T) {
	if testing.Short() {
		t.Skip("waits out the 5s tray grace period")
	}
	// Each case says when a connection to the bus goes silent: before the
	// given connection (counted from 0) sends the given bytes.
	for name, tc := range map[string]struct {
		silent  func(conn int, sent []byte) bool
		warning string
	}{
		"from the start":       {func(int, []byte) bool { return true }, "no system tray found"},
		"after the tray probe": {func(conn int, _ []byte) bool { return conn > 0 }, "no system tray found"},
		"when systray claims its bus name": {func(_ int, sent []byte) bool {
			return bytes.Contains(sent, []byte("RequestName"))
		}, "the tray icon did not start"},
	} {
		t.Run(name, func(t *testing.T) {
			d := newDesktop(t, workingHost)
			sock := filepath.Join(t.TempDir(), "stuck")
			l, err := net.Listen("unix", sock)
			must(t, err)
			var mu sync.Mutex
			var open []io.Closer
			t.Cleanup(func() {
				l.Close()
				mu.Lock()
				defer mu.Unlock()
				for _, c := range open {
					c.Close()
				}
			})
			go func() {
				for i := 0; ; i++ {
					c, err := l.Accept()
					if err != nil {
						return
					}
					real, err := net.Dial("unix", d.bus)
					if err != nil {
						c.Close()
						return
					}
					mu.Lock()
					open = append(open, c, real)
					mu.Unlock()
					go io.Copy(c, real)
					go func(i int) {
						buf := make([]byte, 64<<10)
						for {
							n, err := c.Read(buf)
							if err != nil || tc.silent(i, buf[:n]) {
								return // both sides stay open, unanswered
							}
							if _, err := real.Write(buf[:n]); err != nil {
								return
							}
						}
					}(i)
				}
			}()
			t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+sock)
			cmd, stdout, stderr, exited := startUnsit(t, "--sit", "1s", "--stand", "1s")

			time.Sleep(7500 * time.Millisecond) // the tray gives up after 5s; the first switch follows
			must(t, cmd.Process.Signal(syscall.SIGINT))
			waitExit(t, exited, stderr)
			if !strings.Contains(stderr.String(), tc.warning) {
				t.Errorf("stderr = %q, want %q", stderr, tc.warning)
			}
			if !strings.Contains(stdout.String(), "Stand up: 1 second standing") {
				t.Errorf("stdout = %q, want the cycle running despite the stuck bus", stdout)
			}
		})
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
