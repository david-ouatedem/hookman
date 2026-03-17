package id

import (
	"strings"
	"testing"
)

func TestNewEvent(t *testing.T) {
	id := NewEvent()
	if !strings.HasPrefix(id, "evt_") {
		t.Errorf("event ID should start with 'evt_', got %s", id)
	}
	if len(id) != 30 { // "evt_" (4) + ULID (26)
		t.Errorf("event ID should be 30 chars, got %d: %s", len(id), id)
	}
}

func TestNewEndpoint(t *testing.T) {
	id := NewEndpoint()
	if !strings.HasPrefix(id, "ep_") {
		t.Errorf("endpoint ID should start with 'ep_', got %s", id)
	}
}

func TestNewDeliveryAttempt(t *testing.T) {
	id := NewDeliveryAttempt()
	if !strings.HasPrefix(id, "da_") {
		t.Errorf("delivery attempt ID should start with 'da_', got %s", id)
	}
}

func TestUniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := NewEvent()
		if ids[id] {
			t.Fatalf("duplicate ID generated: %s", id)
		}
		ids[id] = true
	}
}
