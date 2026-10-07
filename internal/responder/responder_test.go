package responder

import (
	"errors"
	"testing"
)

func TestValidate(t *testing.T) {
	bad := []NewResponder{
		{},
		{Type: "helicopter"},
		{Type: Ambulance, Capabilities: []string{"als", ""}},
	}
	for _, in := range bad {
		if err := in.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%+v: expected ErrInvalid, got %v", in, err)
		}
	}
	for _, typ := range validTypes {
		if err := (NewResponder{Type: typ, Capabilities: []string{"x"}}).Validate(); err != nil {
			t.Errorf("%s rejected: %v", typ, err)
		}
	}
}
