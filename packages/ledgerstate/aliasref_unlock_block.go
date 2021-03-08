package ledgerstate

import (
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

// AliasReferencedUnlockBlock defines an UnlockBlock which references a previous UnlockBlock
// The previous can be another AliasReferencedUnlockBlock. This unlock block type allows recursive unlocks
type AliasReferencedUnlockBlock struct {
	referencedIndex uint16
}

// NewAliasUnlockBlock is the constructor for AliasUnlockBlocks.
func NewAliasReferenceUnlockBlock(referencedIndex uint16) *AliasReferencedUnlockBlock {
	return &AliasReferencedUnlockBlock{
		referencedIndex: referencedIndex,
	}
}

// AliasUnlockBlockFromBytes unmarshals a AliasReferencedUnlockBlock from a sequence of bytes.
func AliasReferenceUnlockBlockFromBytes(bytes []byte) (unlockBlock *AliasReferencedUnlockBlock, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if unlockBlock, err = AliasReferenceUnlockBlockFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse AliasReferencedUnlockBlock from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// AliasReferenceUnlockBlockFromMarshalUtil unmarshals a AliasReferencedUnlockBlock using a MarshalUtil (for easier unmarshaling).
func AliasReferenceUnlockBlockFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (unlockBlock *AliasReferencedUnlockBlock, err error) {
	unlockBlockType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("failed to parse UnlockBlockType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if UnlockBlockType(unlockBlockType) != AliasReferencedUnlockBlockType {
		err = xerrors.Errorf("invalid UnlockBlockType (%X): %w", unlockBlockType, cerrors.ErrParseBytesFailed)
		return
	}

	unlockBlock = &AliasReferencedUnlockBlock{}
	if unlockBlock.referencedIndex, err = marshalUtil.ReadUint16(); err != nil {
		err = xerrors.Errorf("failed to parse referencedIndex (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	return
}

// ReferencedIndex returns the index of the referenced UnlockBlock.
func (r *AliasReferencedUnlockBlock) ReferencedIndex() uint16 {
	return r.referencedIndex
}

// Type returns the UnlockBlockType of the UnlockBlock.
func (r *AliasReferencedUnlockBlock) Type() UnlockBlockType {
	return AliasReferencedUnlockBlockType
}

// Bytes returns a marshaled version of the UnlockBlock.
func (r *AliasReferencedUnlockBlock) Bytes() []byte {
	return marshalutil.New(1 + marshalutil.Uint16Size).
		WriteByte(byte(AliasReferencedUnlockBlockType)).
		WriteUint16(r.referencedIndex).
		Bytes()
}

// String returns a human readable version of the UnlockBlock.
func (r *AliasReferencedUnlockBlock) String() string {
	return stringify.Struct("AliasReferencedUnlockBlock",
		stringify.StructField("referencedIndex", int(r.referencedIndex)),
	)
}

// code contract (make sure the type implements all required methods)
var _ UnlockBlock = &AliasReferencedUnlockBlock{}
