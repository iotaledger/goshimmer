package models

import "github.com/pkg/errors"

var (
	// ErrNoStrongParents is triggered if there no strong parents.
	ErrNoStrongParents = errors.New("missing strong blocks in first parent block")

	// ErrBlockTypeIsUnknown is triggered when the block type is unknown.
	ErrBlockTypeIsUnknown = errors.Errorf("block types must range from %d-%d", 1, LastValidBlockType)

	// ErrConflictingReferenceAcrossBlocks is triggered if there conflicting references across blocks.
	ErrConflictingReferenceAcrossBlocks = errors.New("different blocks have conflicting references")
)
