package mana

import "github.com/cockroachdb/errors"

var (
	// ErrAlreadyUpdated is returned if mana is tried to be updated at a later time.
	ErrAlreadyUpdated = errors.New("already updated to a later timestamp")
	// ErrBaseManaNegative is returned if base mana will become negative.
	ErrBaseManaNegative = errors.New("base mana should never be negative")
	// ErrEffBaseManaNegative is returned if base mana will become negative.
	ErrEffBaseManaNegative = errors.New("effective base mana should never be negative")
	// ErrUnknownManaType is returned if mana type could not be identified.
	ErrUnknownManaType = errors.New("unknown mana type")
	// ErrNodeNotFoundInBaseManaVector is returned if the node is not found in the base mana vector.
	ErrNodeNotFoundInBaseManaVector = errors.New("node not present in base mana vector")
	// ErrInvalidWeightParameter is returned if an invalid weight parameter is passed.
	ErrInvalidWeightParameter = errors.New("invalid weight parameter, outside of [0,1]")
	// ErrInvalidTargetManaType is returned if a research base mana vector can't handle the target mana type.
	ErrInvalidTargetManaType = errors.New("invalid target mana type")
	// ErrUnknownManaEvent is returned if mana event type could not be identified.
	ErrUnknownManaEvent = errors.New("unknown mana event")
)
