package timer

import (
	"context"
	"testing"
	"time"
)

const (
	sit   = 40 * time.Minute
	stand = 20 * time.Minute
)

var t0 = time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)

// harness runs the cycle on a fake clock that only moves when the test
// ticks. Setting clock before each send on ticks orders it before Run reads it.
type harness struct {
	t      *testing.T
	clock  time.Time
	ticks  chan time.Time
	states chan State
	cancel context.CancelFunc
	done   chan struct{}
}

func start(t *testing.T) *harness {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	h := &harness{t, t0, make(chan time.Time), make(chan State, 16), cancel, make(chan struct{})}
	go func() {
		defer close(h.done)
		Run(ctx, func() time.Time { return h.clock }, h.ticks, sit, stand, func(s State) { h.states <- s })
	}()
	t.Cleanup(h.stop)
	if got, want := h.next(), (State{Phase: Sitting, Left: sit}); got != want {
		t.Fatalf("initial state = %+v, want %+v", got, want)
	}
	return h
}

// tick moves the clock to t0+at, ticks, and returns the State it produced.
// The tick carries a stale time, which Run must ignore.
func (h *harness) tick(at time.Duration) State {
	h.t.Helper()
	h.clock = t0.Add(at)
	h.ticks <- t0
	return h.next()
}

func (h *harness) next() State {
	h.t.Helper()
	select {
	case s := <-h.states:
		return s
	case <-time.After(time.Second):
		h.t.Fatal("no state from Run")
		return State{}
	}
}

func (h *harness) stop() {
	h.cancel()
	<-h.done
}

func TestPhaseSequence(t *testing.T) {
	h := start(t)
	steps := []struct {
		at   time.Duration
		want State
	}{
		{time.Minute, State{Phase: Sitting, Left: 39 * time.Minute}},
		{sit - time.Second, State{Phase: Sitting, Left: time.Second}},
		{sit, State{Phase: Standing, Left: stand, Switched: true}},
		{sit + time.Minute, State{Phase: Standing, Left: stand - time.Minute}},
		{sit + stand, State{Phase: Sitting, Left: sit, Switched: true}},
		{2*sit + stand, State{Phase: Standing, Left: stand, Switched: true}},
		{2*sit + 2*stand, State{Phase: Sitting, Left: sit, Switched: true}},
	}
	for _, step := range steps {
		if got := h.tick(step.at); got != step.want {
			t.Fatalf("at t0+%v: got %+v, want %+v", step.at, got, step.want)
		}
	}
}

func TestLateTickStartsNextPhaseFromNow(t *testing.T) {
	h := start(t)
	// Five minutes late: standing still gets its full 20 minutes.
	want := State{Phase: Standing, Left: stand, Switched: true}
	if got := h.tick(sit + 5*time.Minute); got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestWakeAfterLongSleepSwitchesOnce(t *testing.T) {
	h := start(t)
	// Asleep for 5 hours: many deadlines passed, but only one switch.
	want := State{Phase: Standing, Left: stand, Switched: true}
	if got := h.tick(5 * time.Hour); got != want {
		t.Fatalf("first tick after wake: got %+v, want %+v", got, want)
	}
	want = State{Phase: Standing, Left: stand - time.Second}
	if got := h.tick(5*time.Hour + time.Second); got != want {
		t.Fatalf("second tick after wake: got %+v, want %+v (backlog?)", got, want)
	}
}

func TestClockSetBackDoesNotStretchPhase(t *testing.T) {
	h := start(t)
	want := State{Phase: Sitting, Left: sit}
	if got := h.tick(-2 * time.Hour); got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestStopInEachPhase(t *testing.T) {
	for _, tc := range []struct {
		phase Phase
		at    time.Duration // a tick that leaves the cycle in phase
		next  time.Duration // a tick past that phase's deadline
	}{
		{Sitting, time.Minute, sit},
		{Standing, sit, sit + stand},
	} {
		t.Run(tc.phase.String(), func(t *testing.T) {
			h := start(t)
			if got := h.tick(tc.at); got.Phase != tc.phase {
				t.Fatalf("setup: in %v, want %v", got.Phase, tc.phase)
			}
			h.stop()
			h.clock = t0.Add(tc.next)
			select {
			case h.ticks <- t0:
				t.Fatal("Run still receiving ticks after stop")
			case <-time.After(50 * time.Millisecond):
			}
			select {
			case s := <-h.states:
				t.Fatalf("update after stop: %+v", s)
			default:
			}
		})
	}
}

// Stopping while a tick is already waiting must not deliver it: whichever of
// the two ready select cases Run picks, nothing more reaches update.
func TestStopRacingPendingTick(t *testing.T) {
	for range 200 {
		ctx, cancel := context.WithCancel(context.Background())
		ticks := make(chan time.Time, 1)
		ticks <- t0
		cancel()
		clock := t0
		updates := 0
		Run(ctx, func() time.Time { return clock }, ticks, sit, stand, func(State) {
			updates++
			clock = t0.Add(sit) // the pending tick would switch to Standing
		})
		if updates != 1 { // only the initial state
			t.Fatalf("got %d updates, want 1", updates)
		}
	}
}
