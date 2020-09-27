package ledgerstate

import (
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

// region UnlockBlockType //////////////////////////////////////////////////////////////////////////////////////////////

const (
	// SignatureUnlockBlockType represents the type of a SignatureUnlockBlock.
	SignatureUnlockBlockType UnlockBlockType = iota

	// ReferenceUnlockBlockType represents the type of a ReferenceUnlockBlock.
	ReferenceUnlockBlockType
)

// UnlockBlockType represents the type of the UnlockBlock. Different types of UnlockBlocks can unlock different types of
// Outputs.
type UnlockBlockType uint8

// String returns a human readable representation of the UnlockBlockType.
func (a UnlockBlockType) String() string {
	return [...]string{
		"SignatureUnlockBlockType",
		"ReferenceUnlockBlockType",
	}[a]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region UnlockBlock //////////////////////////////////////////////////////////////////////////////////////////////////

// UnlockBlock represents the interface to generically addresses different kinds of UnlockBlocks that contain different
// information that can be used to unlock different kinds of Outputs.
type UnlockBlock interface {
	// Type returns the UnlockBlockType of this UnlockBlock.
	Type() UnlockBlockType

	// Bytes returns a marshaled version of this UnlockBlock.
	Bytes() []byte

	// String returns a human readable version of this UnlockBlock.
	String() string
}

// UnlockBlockFromBytes unmarshals an UnlockBlock from a sequence of bytes.
func UnlockBlockFromBytes(bytes []byte) (unlockBlock UnlockBlock, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if unlockBlock, err = UnlockBlockFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse UnlockBlock from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// UnlockBlockFromMarshalUtil unmarshals an UnlockBlock using a MarshalUtil (for easier unmarshaling).
func UnlockBlockFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (unlockBlock UnlockBlock, err error) {
	unlockBlockType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("failed to parse OutputType (%v): %w", err, ErrParseBytesFailed)
		return
	}
	marshalUtil.ReadSeek(-1)

	switch UnlockBlockType(unlockBlockType) {
	case SignatureUnlockBlockType:
		if unlockBlock, err = SignatureUnlockBlockFromMarshalUtil(marshalUtil); err != nil {
			err = xerrors.Errorf("failed to parse SignatureUnlockBlock from MarshalUtil: %w", err)
			return
		}
	case ReferenceUnlockBlockType:
		if unlockBlock, err = ReferenceUnlockBlockFromMarshalUtil(marshalUtil); err != nil {
			err = xerrors.Errorf("failed to parse ReferenceUnlockBlock from MarshalUtil: %w", err)
			return
		}
	default:
		err = xerrors.Errorf("unsupported UnlockBlockType (%X): %w", unlockBlockType, ErrParseBytesFailed)
		return
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SignatureUnlockBlock /////////////////////////////////////////////////////////////////////////////////////////

// SignatureUnlockBlock represents an UnlockBlock that contains a Signature for an Address.
type SignatureUnlockBlock struct {
	signature Signature
}

// NewSignatureUnlockBlock is the constructor for SignatureUnlockBlock objects.
func NewSignatureUnlockBlock(signature Signature) *SignatureUnlockBlock {
	return &SignatureUnlockBlock{
		signature: signature,
	}
}

// SignatureUnlockBlockFromBytes unmarshals a SignatureUnlockBlock from a sequence of bytes.
func SignatureUnlockBlockFromBytes(bytes []byte) (unlockBlock *SignatureUnlockBlock, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if unlockBlock, err = SignatureUnlockBlockFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse SignatureUnlockBlock from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// SignatureUnlockBlockFromMarshalUtil unmarshals a SignatureUnlockBlock using a MarshalUtil (for easier unmarshaling).
func SignatureUnlockBlockFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (unlockBlock *SignatureUnlockBlock, err error) {
	unlockBlockType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("failed to parse UnlockBlockType (%v): %w", err, ErrParseBytesFailed)
		return
	}
	if UnlockBlockType(unlockBlockType) != SignatureUnlockBlockType {
		err = xerrors.Errorf("invalid UnlockBlockType (%X): %w", unlockBlockType, ErrParseBytesFailed)
		return
	}

	unlockBlock = &SignatureUnlockBlock{}
	if unlockBlock.signature, err = SignatureFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Signature from MarshalUtil: %w", err)
		return
	}
	return
}

// AddressSignatureValid returns true if this UnlockBlock correctly signs the given Address.
func (s *SignatureUnlockBlock) AddressSignatureValid(address Address, signedData []byte) (bool, error) {
	return s.signature.SignsAddress(address, signedData), nil
}

// Type returns the UnlockBlockType of this UnlockBlock.
func (s *SignatureUnlockBlock) Type() UnlockBlockType {
	return SignatureUnlockBlockType
}

// Bytes returns a marshaled version of this UnlockBlock.
func (s *SignatureUnlockBlock) Bytes() []byte {
	return byteutils.ConcatBytes([]byte(byte(SignatureUnlockBlockType)), s.signature.Bytes())
}

// String returns a human readable version of this UnlockBlock.
func (s *SignatureUnlockBlock) String() string {
	return stringify.Struct("SignatureUnlockBlock",
		stringify.StructField("signature", s.signature),
	)
}

// code contract (make sure the type implements all required methods)
var _ UnlockBlock = &SignatureUnlockBlock{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ReferenceUnlockBlock /////////////////////////////////////////////////////////////////////////////////////////

// ReferenceUnlockBlock defines an UnlockBlock which references a previous UnlockBlock (which must not be another
// ReferenceUnlockBlock).
type ReferenceUnlockBlock struct {
	referencedIndex uint16
}

// NewReferenceUnlockBlock is the constructor for ReferenceUnlockBlocks.
func NewReferenceUnlockBlock(referencedIndex uint16) *ReferenceUnlockBlock {
	return &ReferenceUnlockBlock{
		referencedIndex: referencedIndex,
	}
}

// ReferenceUnlockBlockFromBytes unmarshals a ReferenceUnlockBlock from a sequence of bytes.
func ReferenceUnlockBlockFromBytes(bytes []byte) (unlockBlock *ReferenceUnlockBlock, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if unlockBlock, err = ReferenceUnlockBlockFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ReferenceUnlockBlock from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ReferenceUnlockBlockFromMarshalUtil unmarshals a ReferenceUnlockBlock using a MarshalUtil (for easier unmarshaling).
func ReferenceUnlockBlockFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (unlockBlock *ReferenceUnlockBlock, err error) {
	unlockBlockType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("failed to parse UnlockBlockType (%v): %w", err, ErrParseBytesFailed)
		return
	}
	if UnlockBlockType(unlockBlockType) != ReferenceUnlockBlockType {
		err = xerrors.Errorf("invalid UnlockBlockType (%X): %w", unlockBlockType, ErrParseBytesFailed)
		return
	}

	unlockBlock = &ReferenceUnlockBlock{}
	if unlockBlock.referencedIndex, err = marshalUtil.ReadUint16(); err != nil {
		err = xerrors.Errorf("failed to parse referencedIndex (%v): %w", err, ErrParseBytesFailed)
		return
	}
	return
}

// Reference returns the index of the referenced UnlockBlock.
func (r *ReferenceUnlockBlock) ReferencedIndex() uint16 {
	return r.referencedIndex
}

// Type returns the UnlockBlockType of this UnlockBlock.
func (r *ReferenceUnlockBlock) Type() UnlockBlockType {
	return ReferenceUnlockBlockType
}

// Bytes returns a marshaled version of this UnlockBlock.
func (r *ReferenceUnlockBlock) Bytes() []byte {
	return marshalutil.New(1 + marshalutil.UINT16_SIZE).
		WriteByte(byte(ReferenceUnlockBlockType)).
		WriteUint16(r.referencedIndex).
		Bytes()
}

// String returns a human readable version of this UnlockBlock.
func (r *ReferenceUnlockBlock) String() string {
	return stringify.Struct("ReferenceUnlockBlock",
		stringify.StructField("referencedIndex", r.referencedIndex),
	)
}

// code contract (make sure the type implements all required methods)
var _ UnlockBlock = &ReferenceUnlockBlock{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
