package main

import (
	"testing"
	"time"

	"unsit/internal/timer"
)

func TestMessages(t *testing.T) {
	c := cycle{sit: 40 * time.Minute, stand: 20 * time.Minute}
	for _, tc := range []struct {
		phase       timer.Phase
		title, body string
	}{
		{timer.Standing, "Stand up", "20 minutes standing"},
		{timer.Sitting, "Sit down", "40 minutes sitting"},
	} {
		if title, body := c.message(tc.phase); title != tc.title || body != tc.body {
			t.Errorf("message(%v) = %q, %q; want %q, %q", tc.phase, title, body, tc.title, tc.body)
		}
	}
}

func TestHuman(t *testing.T) {
	for d, want := range map[time.Duration]string{
		time.Second:                      "1 second",
		10 * time.Second:                 "10 seconds",
		90 * time.Second:                 "90 seconds",
		time.Minute:                      "1 minute",
		45 * time.Minute:                 "45 minutes",
		time.Hour:                        "1 hour",
		90 * time.Minute:                 "90 minutes",
		2 * time.Hour:                    "2 hours",
		1500 * time.Millisecond:          "1.5s",
		time.Hour + 500*time.Millisecond: "1h0m0.5s",
	} {
		if got := human(d); got != want {
			t.Errorf("human(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestFlagErrors(t *testing.T) {
	for _, args := range [][]string{
		{"--sit", "0s"},
		{"--stand", "-5m"},
		{"--sit", "soon"},
		{"--bogus"},
		{"extra"},
	} {
		if code := run(args); code != 2 {
			t.Errorf("run(%q) = %d, want 2", args, code)
		}
	}
}
