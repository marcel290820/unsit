package tray

import "testing"

func TestItemBus(t *testing.T) {
	for item, want := range map[string]string{
		":1.142/StatusNotifierItem":                          ":1.142",
		":1.142@/StatusNotifierItem":                         ":1.142",
		"org.kde.StatusNotifierItem-7-1/StatusNotifierItem":  "org.kde.StatusNotifierItem-7-1",
		"org.kde.StatusNotifierItem-7-1@/StatusNotifierItem": "org.kde.StatusNotifierItem-7-1",
		":1.51@/org/ayatana/NotificationItem/livepatch":      ":1.51",
		":1.9": ":1.9",
	} {
		if got := itemBus(item); got != want {
			t.Errorf("itemBus(%q) = %q; want %q", item, got, want)
		}
	}
}
