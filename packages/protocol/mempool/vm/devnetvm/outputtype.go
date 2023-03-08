package devnetvm

import (
	"fmt"

	"github.com/pkg/errors"
)

// region OutputType ///////////////////////////////////////////////////////////////////////////////////////////////////

// OutputType represents the type of an Output. Outputs of different types can have different unlock rules and allow for
// some relatively basic smart contract logic.
type OutputType uint8

const (
	// SigLockedSingleOutputType represents an Output holding vanilla IOTA tokens that gets unlocked by a signature.
	SigLockedSingleOutputType OutputType = iota

	// SigLockedColoredOutputType represents an Output that holds colored coins that gets unlocked by a signature.
	SigLockedColoredOutputType

	// AliasOutputType represents an Output which makes a chain with optional governance.
	AliasOutputType

	// ExtendedLockedOutputType represents an Output which extends SigLockedColoredOutput with alias locking and fallback.
	ExtendedLockedOutputType
)

// String returns a human readable representation of the OutputType.
func (o OutputType) String() string {
	return [...]string{
		"SigLockedSingleOutputType",
		"SigLockedColoredOutputType",
		"AliasOutputType",
		"ExtendedLockedOutputType",
	}[o]
}

// OutputTypeFromString returns the output type from a string.
func OutputTypeFromString(ot string) (OutputType, error) {
	res, ok := map[string]OutputType{
		"SigLockedSingleOutputType":  SigLockedSingleOutputType,
		"SigLockedColoredOutputType": SigLockedColoredOutputType,
		"AliasOutputType":            AliasOutputType,
		"ExtendedLockedOutputType":   ExtendedLockedOutputType,
	}[ot]
	if !ok {
		return res, errors.New(fmt.Sprintf("unsupported output type: %s", ot))
	}
	return res, nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
