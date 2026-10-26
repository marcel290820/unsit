// Package tray is unsit's tray icon: a seated or standing figure, the time
// left, and a menu with Stop.
package tray

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"time"

	"fyne.io/systray"

	"unsit/internal/timer"
)

var (
	status      *systray.MenuItem
	shownText   string
	shownIcon   []byte
	shortPhases = map[timer.Phase]string{timer.Sitting: "Sit", timer.Standing: "Stand"}
)

// Run shows the tray icon, first displaying first, and blocks until ctx is
// done or the user picks Stop. Once the icon is up, it calls start in a new
// goroutine with a context that also ends on Stop; start keeps the icon
// current with Show.
//
// If there is no tray to show the icon in, Run returns an error without
// calling start. Run must be called from the main goroutine.
func Run(ctx context.Context, first timer.State, start func(context.Context)) error {
	if err := setup(ctx, first); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// systray reports no errors: on Windows, when the icon can't be added
	// (say, Explorer is still busy at login), it never calls back and never
	// returns. So whichever comes first wins: the callback, or giving up.
	const starting, up, gaveUp = 0, 1, 2
	var state atomic.Int32
	giveUp := time.AfterFunc(5*time.Second, func() {
		if state.CompareAndSwap(starting, gaveUp) {
			abort()
		}
	})
	defer giveUp.Stop()

	systray.Run(func() {
		text, _ := describe(first)
		status = systray.AddMenuItem(text, "")
		status.Disable()
		systray.AddSeparator()
		stop := systray.AddMenuItem("Stop", "Stop unsit")
		// Not inline: the Linux menu stays empty until this callback returns.
		go func() {
			if !registered(ctx) || !state.CompareAndSwap(starting, up) {
				return
			}
			started()
			// Quit only once the tray is up; earlier, it can be lost.
			go func() {
				select {
				case <-stop.ClickedCh:
				case <-ctx.Done():
				}
				cancel()
				systray.Quit()
			}()
			start(ctx)
		}()
	}, nil)
	if state.Load() == gaveUp {
		return errors.New("the tray icon did not start")
	}
	return nil
}

// Show displays s: the icon for its phase, and the time left in the menu,
// the tooltip and, on macOS, the menu bar. Call it from one goroutine only.
func Show(s timer.State) {
	text, title := describe(s)
	if text == shownText {
		return
	}
	shownText = text
	status.SetTitle(text)
	systray.SetTooltip(text)
	if runtime.GOOS == "darwin" {
		systray.SetTitle(title)
	}
	// Checked once a minute, so a bar switching themes gets the right icon soon.
	if icon := iconFor(s.Phase); !bytes.Equal(icon, shownIcon) {
		shownIcon = icon
		setIcon(icon)
	}
}

// describe returns the status line for the menu and tooltip, and the short
// title for the macOS menu bar.
func describe(s timer.State) (text, title string) {
	mins := int((s.Left + time.Minute - 1) / time.Minute) // rounded up: never "0 min left"
	return fmt.Sprintf("%s · %d min left", s.Phase, mins), fmt.Sprintf("%s %dm", shortPhases[s.Phase], mins)
}
