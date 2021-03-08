package ledgerstate

import (
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

// AliasReferenceUnlockBlock defines an UnlockBlock which references a previous UnlockBlock
// The previous can be another AliasReferenceUnlockBlock. This unlock block type allows recursive unlocks
type AliasReferenceUnlockBlock struct {
	referencedIndex uint16
}

// NewAliasUnlockBlock is the constructor for AliasUnlockBlocks.
func NewAliasReferenceUnlockBlock(referencedIndex uint16) *AliasReferenceUnlockBlock {
	return &AliasReferenceUnlockBlock{
		referencedIndex: referencedIndex,
	}
}

// AliasUnlockBlockFromBytes unmarshals a AliasReferenceUnlockBlock from a sequence of bytes.
func AliasReferenceUnlockBlockFromBytes(bytes []byte) (unlockBlock *AliasReferenceUnlockBlock, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if unlockBlock, err = AliasReferenceUnlockBlockFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse AliasReferenceUnlockBlock from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// AliasReferenceUnlockBlockFromMarshalUtil unmarshals a AliasReferenceUnlockBlock using a MarshalUtil (for easier unmarshaling).
func AliasReferenceUnlockBlockFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (unlockBlock *AliasReferenceUnlockBlock, err error) {
	unlockBlockType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("failed to parse UnlockBlockType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if UnlockBlockType(unlockBlockType) != AliasUnlockBlockType {
		err = xerrors.Errorf("invalid UnlockBlockType (%X): %w", unlockBlockType, cerrors.ErrParseBytesFailed)
		return
	}

	unlockBlock = &AliasReferenceUnlockBlock{}
	if unlockBlock.referencedIndex, err = marshalUtil.ReadUint16(); err != nil {
		err = xerrors.Errorf("failed to parse referencedIndex (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	return
}

// ReferencedIndex returns the index of the referenced UnlockBlock.
func (r *AliasReferenceUnlockBlock) ReferencedIndex() uint16 {
	return r.referencedIndex
}

// Type returns the UnlockBlockType of the UnlockBlock.
func (r *AliasReferenceUnlockBlock) Type() UnlockBlockType {
	return AliasUnlockBlockType
}

// Bytes returns a marshaled version of the UnlockBlock.
func (r *AliasReferenceUnlockBlock) Bytes() []byte {
	return marshalutil.New(1 + marshalutil.Uint16Size).
		WriteByte(byte(AliasUnlockBlockType)).
		WriteUint16(r.referencedIndex).
		Bytes()
}

// String returns a human readable version of the UnlockBlock.
func (r *AliasReferenceUnlockBlock) String() string {
	return stringify.Struct("AliasReferenceUnlockBlock",
		stringify.StructField("referencedIndex", int(r.referencedIndex)),
	)
}

// UnlockValid checks if the AliasReferenceUnlockBlock unlocks input in the context of all inputs
func (r *AliasReferenceUnlockBlock) UnlockValid(input Output, tx *Transaction, allInputs Outputs) (bool, error) {
	if int(r.referencedIndex) > len(allInputs) || input {
		return false, xerrors.New("wrong index")
	}
	return false, nil
}

// code contract (make sure the type implements all required methods)
var _ UnlockBlock = &AliasReferenceUnlockBlock{}
