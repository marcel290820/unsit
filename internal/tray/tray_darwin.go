package tray

import (
	"context"

	"fyne.io/systray"

	"unsit/assets"
	"unsit/internal/timer"
)

func setup(context.Context, timer.State) error { return nil }

// The icon is in the menu bar by the time systray calls back.
func registered(context.Context) bool { return true }

func started() {}

func abort() { systray.Quit() }

func iconFor(p timer.Phase) []byte {
	if p == timer.Standing {
		return assets.StandBlackPNG
	}
	return assets.SitBlackPNG
}

// A template image: macOS tints it to suit a light or dark menu bar.
func setIcon(icon []byte) { systray.SetTemplateIcon(icon, icon) }
