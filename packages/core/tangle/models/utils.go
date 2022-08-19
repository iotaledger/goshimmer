package models

import (
	"context"
	"fmt"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/serix"
	"github.com/iotaledger/hive.go/serializer/v2"

	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"
)

func IsEmptyBlockID(blockID BlockID) bool {
	return blockID == EmptyBlockID
}

var EmptyBlock *Block

func init() {
	EmptyBlock = NewEmptyBlock(EmptyBlockID)
	EmptyBlock.M.PayloadBytes = lo.PanicOnErr(payload.NewGenericDataPayload([]byte("")).Bytes())

	blockIDsArrayRules := &serix.ArrayRules{
		Min:            MinParentsCount,
		Max:            MaxParentsCount,
		ValidationMode: serializer.ArrayValidationModeNoDuplicates,
	}
	err := serix.DefaultAPI.RegisterTypeSettings(BlockIDs{}, serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte).WithArrayRules(blockIDsArrayRules))
	if err != nil {
		panic(fmt.Errorf("error registering BlockIDs type settings: %w", err))
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
		panic(fmt.Errorf("error registering ParentBlockIDs type settings: %w", err))
	}
	err = serix.DefaultAPI.RegisterValidators(ParentBlockIDs{}, validateParentBlockIDsBytes, validateParentBlockIDs)

	if err != nil {
		panic(fmt.Errorf("error registering ParentBlockIDs validators: %w", err))
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
