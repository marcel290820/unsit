// Package timer is the sit/stand cycle: a pure state machine driven by an
// injected clock, with no OS code.
package timer

import (
	"context"
	"time"
)

// Phase is what you should be doing right now.
type Phase int

const (
	Sitting Phase = iota
	Standing
)

func (p Phase) String() string {
	if p == Standing {
		return "Standing"
	}
	return "Sitting"
}

// State is one observation of the cycle.
type State struct {
	Phase Phase
	Left  time.Duration // until the next switch; always positive
	// Switched marks the first State of a new phase. The initial Sitting
	// phase is not a switch.
	Switched bool
}

// Run starts the cycle in Sitting and passes update the initial State, then
// one State per tick, reading the time from now. Tick values are ignored: a
// ticker can deliver a tick long after it was due, stamped with the old time.
//
// Phase ends are wall-clock deadlines, so time spent asleep counts. A tick
// past the deadline switches phase exactly once, however late it is, and the
// new phase runs its full length from that tick: no backlog of switches.
//
// Run returns when ctx is done or ticks is closed, and never calls update
// after ctx is done.
func Run(ctx context.Context, now func() time.Time, ticks <-chan time.Time, sit, stand time.Duration, update func(State)) {
	length := func(p Phase) time.Duration {
		if p == Standing {
			return stand
		}
		return sit
	}

	phase := Sitting
	// Round(0) drops the monotonic reading, which stops while the machine
	// sleeps on some systems, so all math here is wall-clock.
	deadline := now().Round(0).Add(sit)
	update(State{Phase: phase, Left: sit})

	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ticks:
			// Both cases can be ready at once; stopping wins.
			if !ok || ctx.Err() != nil {
				return
			}
			t := now().Round(0)
			switched := !t.Before(deadline)
			if switched {
				phase = 1 - phase
			}
			// The second case catches the clock being set back, which would
			// otherwise stretch the phase.
			if switched || deadline.Sub(t) > length(phase) {
				deadline = t.Add(length(phase))
			}
			update(State{Phase: phase, Left: deadline.Sub(t), Switched: switched})
		}
	}
}
