package tangleold

import "github.com/cockroachdb/errors"

var (
	// ErrNotBootstrapped is triggered when somebody tries to issue a Payload before the Tangle is fully bootstrapped.
	ErrNotBootstrapped = errors.New("tangle not bootstrapped")
	// ErrParentsInvalid is returned when one or more parents of a block is invalid.
	ErrParentsInvalid = errors.New("one or more parents is invalid")
)
