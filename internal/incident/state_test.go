package incident

import (
	"errors"
	"testing"
)

func TestHappyPath(t *testing.T) {
	path := []Status{Reported, Triaged, Dispatched, Acknowledged, EnRoute, OnScene, Resolved}
	for i := 1; i < len(path); i++ {
		if err := CanTransition(path[i-1], path[i]); err != nil {
			t.Errorf("%s -> %s: %v", path[i-1], path[i], err)
		}
	}
}

func TestReassignmentLoop(t *testing.T) {
	for _, from := range []Status{Dispatched, Acknowledged, EnRoute} {
		if err := CanTransition(from, Reassigned); err != nil {
			t.Errorf("%s -> REASSIGNED: %v", from, err)
		}
	}
	if err := CanTransition(Reassigned, Dispatched); err != nil {
		t.Errorf("REASSIGNED -> DISPATCHED: %v", err)
	}
}

func TestIllegalTransitions(t *testing.T) {
	cases := []struct{ from, to Status }{
		{Reported, Dispatched}, // skips triage
		{Reported, Resolved},
		{Triaged, OnScene},
		{OnScene, Reassigned}, // already on scene, too late to reassign
		{Resolved, Reported},  // terminal
		{Resolved, Resolved},
		{Dispatched, Triaged}, // backwards
		{Reassigned, Acknowledged},
		{Status("BOGUS"), Triaged},
		{Reported, Status("BOGUS")},
	}
	for _, c := range cases {
		err := CanTransition(c.from, c.to)
		var illegal ErrIllegalTransition
		if !errors.As(err, &illegal) {
			t.Errorf("%s -> %s: expected ErrIllegalTransition, got %v", c.from, c.to, err)
		}
	}
}

func TestEveryStatusHasAnEntry(t *testing.T) {
	for from, nexts := range transitions {
		for _, to := range nexts {
			if !to.Valid() {
				t.Errorf("%s lists unknown target %s", from, to)
			}
		}
	}
}
