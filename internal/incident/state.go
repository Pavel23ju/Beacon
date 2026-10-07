package incident

import "fmt"

// Status is a stage in the incident lifecycle.
type Status string

const (
	Reported     Status = "REPORTED"
	Triaged      Status = "TRIAGED"
	Dispatched   Status = "DISPATCHED"
	Acknowledged Status = "ACKNOWLEDGED"
	EnRoute      Status = "EN_ROUTE"
	OnScene      Status = "ON_SCENE"
	Resolved     Status = "RESOLVED"
	// Reassigned means the current responder was withdrawn (ack timeout or
	// preemption) and the incident must be dispatched again.
	Reassigned Status = "REASSIGNED"
)

// transitions lists every legal move. Anything not listed is rejected.
var transitions = map[Status][]Status{
	Reported:     {Triaged},
	Triaged:      {Dispatched},
	Dispatched:   {Acknowledged, Reassigned},
	Acknowledged: {EnRoute, Reassigned},
	EnRoute:      {OnScene, Reassigned},
	OnScene:      {Resolved},
	Reassigned:   {Dispatched},
	Resolved:     {},
}

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	_, ok := transitions[s]
	return ok
}

// ErrIllegalTransition is returned when a move is not in the transition table.
type ErrIllegalTransition struct{ From, To Status }

func (e ErrIllegalTransition) Error() string {
	return fmt.Sprintf("illegal transition %s -> %s", e.From, e.To)
}

// CanTransition validates a move from one status to another.
func CanTransition(from, to Status) error {
	for _, next := range transitions[from] {
		if next == to {
			return nil
		}
	}
	return ErrIllegalTransition{From: from, To: to}
}
