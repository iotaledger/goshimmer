package models

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hive.go/serializer/v2/serix"
)

func init() {
	blockIDsArrayRules := &serix.ArrayRules{
		Min:            MinParentsCount,
		Max:            MaxParentsCount,
		ValidationMode: serializer.ArrayValidationModeNoDuplicates,
	}
	err := serix.DefaultAPI.RegisterTypeSettings(BlockIDs{}, serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte).WithArrayRules(blockIDsArrayRules))
	if err != nil {
		panic(errors.Wrap(err, "error registering BlockIDs type settings"))
	}
	parentsBlockIDsArrayRules := &serix.ArrayRules{
		Min:            MinParentsBlocksCount,
		Max:            MaxParentsBlocksCount,
		ValidationMode: serializer.ArrayValidationModeNoDuplicates,
		UniquenessSliceFunc: func(next []byte) []byte {
			// return first byte which indicates the parent type
			return next[:1]
		},
	}
	err = serix.DefaultAPI.RegisterTypeSettings(ParentBlockIDs{}, serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte).WithArrayRules(parentsBlockIDsArrayRules))
	if err != nil {
		panic(errors.Wrap(err, "error registering ParentBlockIDs type settings"))
	}
	err = serix.DefaultAPI.RegisterValidators(ParentBlockIDs{}, validateParentBlockIDsBytes, validateParentBlockIDs)

	if err != nil {
		panic(errors.Wrap(err, "error registering ParentBlockIDs validators"))
	}
}

func validateParentBlockIDs(_ context.Context, parents ParentBlockIDs) (err error) {
	// Validate strong parent block
	if strongParents, strongParentsExist := parents[StrongParentType]; len(parents) == 0 || !strongParentsExist ||
		len(strongParents) < MinStrongParentsCount {
		return ErrNoStrongParents
	}
	for parentsType := range parents {
		if parentsType > LastValidBlockType {
			return ErrBlockTypeIsUnknown
		}
	}
	if areReferencesConflictingAcrossBlocks(parents) {
		return ErrConflictingReferenceAcrossBlocks
	}
	return nil
}

// validate blocksIDs are unique across blocks
// there may be repetition across strong and like parents.
func areReferencesConflictingAcrossBlocks(parentsBlocks ParentBlockIDs) bool {
	for blockID := range parentsBlocks[WeakParentType] {
		if _, exists := parentsBlocks[StrongParentType][blockID]; exists {
			return true
		}

		if _, exists := parentsBlocks[ShallowLikeParentType][blockID]; exists {
			return true
		}
	}

	return false
}

func validateParentBlockIDsBytes(_ context.Context, _ []byte) (err error) {
	return
}
