package tray

import (
	"testing"
	"time"

	"unsit/internal/timer"
)

func TestDescribe(t *testing.T) {
	for _, tc := range []struct {
		s           timer.State
		text, title string
	}{
		{timer.State{Phase: timer.Sitting, Left: 40 * time.Minute}, "Sitting · 40 min left", "Sit 40m"},
		{timer.State{Phase: timer.Sitting, Left: 22*time.Minute + time.Second}, "Sitting · 23 min left", "Sit 23m"},
		{timer.State{Phase: timer.Standing, Left: 7 * time.Minute}, "Standing · 7 min left", "Stand 7m"},
		{timer.State{Phase: timer.Standing, Left: 30 * time.Second}, "Standing · 1 min left", "Stand 1m"},
	} {
		if text, title := describe(tc.s); text != tc.text || title != tc.title {
			t.Errorf("describe(%+v) = %q, %q; want %q, %q", tc.s, text, title, tc.text, tc.title)
		}
	}
}
