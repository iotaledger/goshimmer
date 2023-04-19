package conflictdag

import (
	"github.com/iotaledger/hive.go/constraints"
)

// IDType is the constraint for the identifier of a conflict or a resource.
type IDType interface {
	// comparable is a built-in constraint that ensures that the type can be used as a map key.
	comparable

	// Bytes returns a serialized version of the ID.
	Bytes() ([]byte, error)

	// String returns a human-readable version of the ID.
	String() string
}

// VotePowerType is the constraint for the vote power of a voter.
type VotePowerType[T any] interface {
	// Comparable imports the constraints.Comparable[T] interface to ensure that the type can be compared.
	constraints.Comparable[T]

	// Increase returns the next higher value of the current value.
	Increase() T

	// Decrease returns the next lower value of the current value.
	Decrease() T
}
