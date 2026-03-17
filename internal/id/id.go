package id

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
)

func newULID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// NewEvent generates a new event ID with "evt_" prefix.
func NewEvent() string {
	return fmt.Sprintf("evt_%s", newULID())
}

// NewEndpoint generates a new endpoint ID with "ep_" prefix.
func NewEndpoint() string {
	return fmt.Sprintf("ep_%s", newULID())
}

// NewDeliveryAttempt generates a new delivery attempt ID with "da_" prefix.
func NewDeliveryAttempt() string {
	return fmt.Sprintf("da_%s", newULID())
}
