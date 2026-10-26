// Command unsit alternates sitting and standing: a gentle notification at
// each switch, a tray icon showing the time left, and nothing else.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"unsit/internal/notify"
	"unsit/internal/timer"
	"unsit/internal/tray"
)

var version = "dev" // set at release build time

const usage = `unsit alternates sitting and standing, with a notification at each switch.

Usage:
  unsit [flags]

Flags:
  --sit 40m      how long to sit (Go durations: 40m, 1h, 1h30m, 10s)
  --stand 20m    how long to stand
  --no-tray      no tray icon; print a line at each switch instead
  --version      print the version
  --help         print this help

Stop it from the tray menu, or with Ctrl-C.
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	flags := flag.NewFlagSet("unsit", flag.ContinueOnError)
	flags.SetOutput(io.Discard) // errors are reported below, help is ours
	sit := flags.Duration("sit", 40*time.Minute, "")
	stand := flags.Duration("stand", 20*time.Minute, "")
	noTray := flags.Bool("no-tray", false, "")
	showVersion := flags.Bool("version", false, "")

	err := flags.Parse(args)
	switch {
	case errors.Is(err, flag.ErrHelp):
		fmt.Print(usage)
		return 0
	case err != nil:
		return fail(err.Error())
	case flags.NArg() > 0:
		return fail(fmt.Sprintf("unexpected argument %q", flags.Arg(0)))
	case *sit < time.Second || *stand < time.Second:
		return fail("--sit and --stand must be at least 1s")
	case *showVersion:
		fmt.Println("unsit", version)
		return 0
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	context.AfterFunc(ctx, stop) // a second Ctrl-C ends a stuck shutdown

	c := cycle{sit: *sit, stand: *stand}
	if !*noTray {
		first := timer.State{Phase: timer.Sitting, Left: *sit}
		err := tray.Run(ctx, first, func(ctx context.Context) {
			c.run(ctx, func(s timer.State) {
				c.notify(ctx, s)
				tray.Show(s)
			})
		})
		if err == nil || ctx.Err() != nil {
			return 0
		}
		fmt.Fprintf(os.Stderr, "unsit: %v; continuing without a tray icon\n", err)
	}

	fmt.Printf("%s  Sitting for %s. Stop with Ctrl-C.\n", clock(), human(*sit))
	c.run(ctx, func(s timer.State) {
		if s.Switched {
			title, body := c.message(s.Phase)
			fmt.Printf("%s  %s: %s\n", clock(), title, body)
		}
		c.notify(ctx, s)
	})
	return 0
}

func fail(msg string) int {
	fmt.Fprintf(os.Stderr, "unsit: %s\nRun 'unsit --help' for usage.\n", msg)
	return 2
}

type cycle struct{ sit, stand time.Duration }

// run drives the cycle on the real clock until ctx is done.
func (c cycle) run(ctx context.Context, update func(timer.State)) {
	timer.Run(ctx, time.Now, time.Tick(time.Second), c.sit, c.stand, update)
}

// message is the notification for entering phase p.
func (c cycle) message(p timer.Phase) (title, body string) {
	if p == timer.Standing {
		return "Stand up", human(c.stand) + " standing"
	}
	return "Sit down", human(c.sit) + " sitting"
}

func (c cycle) notify(ctx context.Context, s timer.State) {
	if !s.Switched {
		return
	}
	// The cycle waits for the notification, so never long, and stopping cancels it.
	sendCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	title, body := c.message(s.Phase)
	if err := notify.Send(sendCtx, title, body); err != nil && ctx.Err() == nil {
		fmt.Fprintln(os.Stderr, "unsit: notification failed:", err)
	}
}

// human spells out a duration for people: "40 minutes", "1 hour", "10 seconds".
func human(d time.Duration) string {
	n, unit := d/time.Second, "second"
	switch {
	case d%time.Hour == 0:
		n, unit = d/time.Hour, "hour"
	case d%time.Minute == 0:
		n, unit = d/time.Minute, "minute"
	case d%time.Second != 0:
		return d.String()
	}
	if n != 1 {
		unit += "s"
	}
	return fmt.Sprintf("%d %s", n, unit)
}

func clock() string { return time.Now().Format("15:04") }
