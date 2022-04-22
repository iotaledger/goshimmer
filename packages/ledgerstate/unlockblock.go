package ledgerstate

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/bytesfilter"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"
)

//nolint:dupl
func init() {
	err := serix.DefaultAPI.RegisterTypeSettings(new(AliasUnlockBlock), serix.TypeSettings{}.WithObjectType(uint8(new(AliasUnlockBlock).Type())))
	if err != nil {
		panic(fmt.Errorf("error registering AliasUnlockBlock type settings: %w", err))
	}
	err = serix.DefaultAPI.RegisterTypeSettings(new(ReferenceUnlockBlock), serix.TypeSettings{}.WithObjectType(uint8(new(ReferenceUnlockBlock).Type())))
	if err != nil {
		panic(fmt.Errorf("error registering ReferenceUnlockBlock type settings: %w", err))
	}
	err = serix.DefaultAPI.RegisterTypeSettings(new(SignatureUnlockBlock), serix.TypeSettings{}.WithObjectType(uint8(new(SignatureUnlockBlock).Type())))
	if err != nil {
		panic(fmt.Errorf("error registering SignatureUnlockBlock type settings: %w", err))
	}
	err = serix.DefaultAPI.RegisterInterfaceObjects((*UnlockBlock)(nil), new(AliasUnlockBlock), new(ReferenceUnlockBlock), new(SignatureUnlockBlock))
	if err != nil {
		panic(fmt.Errorf("error registering UnlockBlock interface implementations: %w", err))
	}
}

// region UnlockBlockType. //////////////////////////////////////////////////////////////////////////////////////////////
const (
	// SignatureUnlockBlockType represents the type of a SignatureUnlockBlock.
	SignatureUnlockBlockType UnlockBlockType = iota

	// ReferenceUnlockBlockType represents the type of a ReferenceUnlockBlock.
	ReferenceUnlockBlockType

	// AliasUnlockBlockType represents the type of a AliasUnlockBlock.
	AliasUnlockBlockType
)

// UnlockBlockType represents the type of the UnlockBlock. Different types of UnlockBlocks can unlock different types of
// Outputs.
type UnlockBlockType uint8

// String returns a human readable representation of the UnlockBlockType.
func (a UnlockBlockType) String() string {
	return [...]string{
		"SignatureUnlockBlockType",
		"ReferenceUnlockBlockType",
		"AliasUnlockBlockType",
	}[a]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region UnlockBlock //////////////////////////////////////////////////////////////////////////////////////////////////

// UnlockBlock represents a generic interface to address the different kinds of unlock information that are required to
// authorize the spending of different Output types.
type UnlockBlock interface {
	// Type returns the UnlockBlockType of the UnlockBlock.
	Type() UnlockBlockType

	// Bytes returns a marshaled version of the UnlockBlock.
	Bytes() []byte

	// String returns a human readable version of the UnlockBlock.
	String() string
}

// UnlockBlockFromBytes unmarshals an UnlockBlock from a sequence of bytes.
func UnlockBlockFromBytes(bytes []byte) (unlockBlock UnlockBlock, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if unlockBlock, err = UnlockBlockFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse UnlockBlock from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// UnlockBlockFromMarshalUtil unmarshals an UnlockBlock using a MarshalUtil (for easier unmarshaling).
func UnlockBlockFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (unlockBlock UnlockBlock, err error) {
	unlockBlockType, err := marshalUtil.ReadByte()
	if err != nil {
		err = errors.Errorf("failed to parse UnlockBlockType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	marshalUtil.ReadSeek(-1)

	switch UnlockBlockType(unlockBlockType) {
	case SignatureUnlockBlockType:
		if unlockBlock, err = SignatureUnlockBlockFromMarshalUtil(marshalUtil); err != nil {
			err = errors.Errorf("failed to parse SignatureUnlockBlock from MarshalUtil: %w", err)
			return
		}
	case ReferenceUnlockBlockType:
		if unlockBlock, err = ReferenceUnlockBlockFromMarshalUtil(marshalUtil); err != nil {
			err = errors.Errorf("failed to parse ReferenceUnlockBlock from MarshalUtil: %w", err)
			return
		}
	case AliasUnlockBlockType:
		if unlockBlock, err = AliasUnlockBlockFromMarshalUtil(marshalUtil); err != nil {
			err = errors.Errorf("failed to parse AliasUnlockBlock from MarshalUtil: %w", err)
			return
		}

	default:
		err = errors.Errorf("unsupported UnlockBlockType (%X): %w", unlockBlockType, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region UnlockBlocks /////////////////////////////////////////////////////////////////////////////////////////////////

// UnlockBlocks is slice of UnlockBlocks that offers additional methods for easier marshaling and unmarshaling.
type UnlockBlocks []UnlockBlock

// UnlockBlocksFromBytes unmarshals UnlockBlocks from a sequence of bytes.
func UnlockBlocksFromBytes(bytes []byte) (unlockBlocks UnlockBlocks, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if unlockBlocks, err = UnlockBlocksFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse UnlockBlocks from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// UnlockBlocksFromMarshalUtil unmarshals UnlockBlocks using a MarshalUtil (for easier unmarshaling).
func UnlockBlocksFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (unlockBlocks UnlockBlocks, err error) {
	unlockBlockCount, err := marshalUtil.ReadUint16()
	if err != nil {
		err = errors.Errorf("failed to parse UnlockBlock count (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	seenUnlockBlocks := bytesfilter.New(int(unlockBlockCount))
	unlockBlocks = make(UnlockBlocks, unlockBlockCount)
	for i := uint16(0); i < unlockBlockCount; i++ {
		unlockBlockBytesStart := marshalUtil.ReadOffset()
		unlockBlock, unlockBlockErr := UnlockBlockFromMarshalUtil(marshalUtil)
		if unlockBlockErr != nil {
			err = errors.Errorf("failed to parse UnlockBlock from MarshalUtil: %w", unlockBlockErr)
			return
		}

		unlockBlockBytes, unlockBlockBytesErr := marshalUtil.ReadBytes(marshalUtil.ReadOffset()-unlockBlockBytesStart, unlockBlockBytesStart)
		if unlockBlockBytesErr != nil {
			err = errors.Errorf("failed to parse UnlockBlock bytes from MarshalUtil: %w", unlockBlockBytesErr)
			return
		}

		if unlockBlock.Type() != ReferenceUnlockBlockType &&
			unlockBlock.Type() != AliasUnlockBlockType &&
			!seenUnlockBlocks.Add(unlockBlockBytes) {
			err = errors.Errorf("duplicate UnlockBlock detected at index %d: %w", i, cerrors.ErrParseBytesFailed)
			return
		}

		unlockBlocks[i] = unlockBlock
	}

	return
}

// Bytes returns a marshaled version of the UnlockBlocks.
func (u UnlockBlocks) Bytes() []byte {
	marshalUtil := marshalutil.New()
	marshalUtil.WriteUint16(uint16(len(u)))
	for _, unlockBlock := range u {
		marshalUtil.WriteBytes(unlockBlock.Bytes())
	}

	return marshalUtil.Bytes()
}

// String returns a human readable version of the UnlockBlocks.
func (u UnlockBlocks) String() string {
	structBuilder := stringify.StructBuilder("UnlockBlocks")
	for i, unlockBlock := range u {
		structBuilder.AddField(stringify.StructField(strconv.Itoa(i), unlockBlock))
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SignatureUnlockBlock /////////////////////////////////////////////////////////////////////////////////////////

// SignatureUnlockBlock represents an UnlockBlock that contains a Signature for an Address.
type SignatureUnlockBlock struct {
	signatureUnlockBlockInner `serix:"0"`
}
type signatureUnlockBlockInner struct {
	Signature Signature `serix:"0"`
}

// NewSignatureUnlockBlock is the constructor for SignatureUnlockBlock objects.
func NewSignatureUnlockBlock(signature Signature) *SignatureUnlockBlock {
	return &SignatureUnlockBlock{
		signatureUnlockBlockInner{
			Signature: signature,
		},
	}
}

// SignatureUnlockBlockFromBytes unmarshals a SignatureUnlockBlock from a sequence of bytes.
func SignatureUnlockBlockFromBytes(bytes []byte) (unlockBlock *SignatureUnlockBlock, consumedBytes int, err error) {
	_, err = serix.DefaultAPI.Decode(context.Background(), bytes, unlockBlock, serix.WithValidation())
	if err != nil {
		return nil, consumedBytes, err
	}
	return
}

// SignatureUnlockBlockFromBytes unmarshals a SignatureUnlockBlock from a sequence of bytes.
func SignatureUnlockBlockFromBytesOld(bytes []byte) (unlockBlock *SignatureUnlockBlock, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if unlockBlock, err = SignatureUnlockBlockFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse SignatureUnlockBlock from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// SignatureUnlockBlockFromMarshalUtil unmarshals a SignatureUnlockBlock using a MarshalUtil (for easier unmarshaling).
func SignatureUnlockBlockFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (unlockBlock *SignatureUnlockBlock, err error) {
	unlockBlockType, err := marshalUtil.ReadByte()
	if err != nil {
		err = errors.Errorf("failed to parse UnlockBlockType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if UnlockBlockType(unlockBlockType) != SignatureUnlockBlockType {
		err = errors.Errorf("invalid UnlockBlockType (%X): %w", unlockBlockType, cerrors.ErrParseBytesFailed)
		return
	}

	unlockBlock = &SignatureUnlockBlock{}
	if unlockBlock.signatureUnlockBlockInner.Signature, err = SignatureFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse Signature from MarshalUtil: %w", err)
		return
	}
	return
}

// AddressSignatureValid returns true if the UnlockBlock correctly signs the given Address.
func (s *SignatureUnlockBlock) AddressSignatureValid(address Address, signedData []byte) bool {
	return s.signatureUnlockBlockInner.Signature.AddressSignatureValid(address, signedData)
}

// Type returns the UnlockBlockType of the UnlockBlock.
func (s *SignatureUnlockBlock) Type() UnlockBlockType {
	return SignatureUnlockBlockType
}

// Bytes returns a marshaled version of the UnlockBlock.
func (s *SignatureUnlockBlock) Bytes() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), s, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		return nil
	}
	return objBytes
}

// Bytes returns a marshaled version of the UnlockBlock.
func (s *SignatureUnlockBlock) BytesOld() []byte {
	return byteutils.ConcatBytes([]byte{byte(SignatureUnlockBlockType)}, s.signatureUnlockBlockInner.Signature.Bytes())
}

// String returns a human readable version of the UnlockBlock.
func (s *SignatureUnlockBlock) String() string {
	return stringify.Struct("SignatureUnlockBlock",
		stringify.StructField("signature", s.signatureUnlockBlockInner.Signature),
	)
}

// Signature return the signature itself.
func (s *SignatureUnlockBlock) Signature() Signature {
	return s.signatureUnlockBlockInner.Signature
}

// code contract (make sure the type implements all required methods)
var _ UnlockBlock = &SignatureUnlockBlock{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ReferenceUnlockBlock /////////////////////////////////////////////////////////////////////////////////////////

// ReferenceUnlockBlock defines an UnlockBlock which references a previous UnlockBlock (which must not be another
// ReferenceUnlockBlock).
type ReferenceUnlockBlock struct {
	referenceUnlockBlockInner `serix:"0"`
}
type referenceUnlockBlockInner struct {
	ReferencedIndex uint16 `serix:"0"`
}

// NewReferenceUnlockBlock is the constructor for ReferenceUnlockBlocks.
func NewReferenceUnlockBlock(referencedIndex uint16) *ReferenceUnlockBlock {
	return &ReferenceUnlockBlock{
		referenceUnlockBlockInner{
			ReferencedIndex: referencedIndex,
		},
	}
}

// ReferenceUnlockBlockFromBytes unmarshals a ReferenceUnlockBlock from a sequence of bytes.
func ReferenceUnlockBlockFromBytes(bytes []byte) (unlockBlock *ReferenceUnlockBlock, consumedBytes int, err error) {
	_, err = serix.DefaultAPI.Decode(context.Background(), bytes, unlockBlock, serix.WithValidation())
	if err != nil {
		return nil, consumedBytes, err
	}
	return
}

// ReferenceUnlockBlockFromBytes unmarshals a ReferenceUnlockBlock from a sequence of bytes.
func ReferenceUnlockBlockFromBytesOld(bytes []byte) (unlockBlock *ReferenceUnlockBlock, consumedBytes int, err error) {
	// TODO: remove eventually
	marshalUtil := marshalutil.New(bytes)
	if unlockBlock, err = ReferenceUnlockBlockFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse ReferenceUnlockBlock from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ReferenceUnlockBlockFromMarshalUtil unmarshals a ReferenceUnlockBlock using a MarshalUtil (for easier unmarshaling).
func ReferenceUnlockBlockFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (unlockBlock *ReferenceUnlockBlock, err error) {
	unlockBlockType, err := marshalUtil.ReadByte()
	if err != nil {
		err = errors.Errorf("failed to parse UnlockBlockType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if UnlockBlockType(unlockBlockType) != ReferenceUnlockBlockType {
		err = errors.Errorf("invalid UnlockBlockType (%X): %w", unlockBlockType, cerrors.ErrParseBytesFailed)
		return
	}

	unlockBlock = &ReferenceUnlockBlock{}
	if unlockBlock.referenceUnlockBlockInner.ReferencedIndex, err = marshalUtil.ReadUint16(); err != nil {
		err = errors.Errorf("failed to parse ReferencedIndex (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	return
}

// ReferencedIndex returns the index of the referenced UnlockBlock.
func (r *ReferenceUnlockBlock) ReferencedIndex() uint16 {
	return r.referenceUnlockBlockInner.ReferencedIndex
}

// Type returns the UnlockBlockType of the UnlockBlock.
func (r *ReferenceUnlockBlock) Type() UnlockBlockType {
	return ReferenceUnlockBlockType
}

// Bytes returns a marshaled version of the Address.
func (r *ReferenceUnlockBlock) Bytes() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), r, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		return nil
	}
	return objBytes
}

// Bytes returns a marshaled version of the UnlockBlock.
func (r *ReferenceUnlockBlock) BytesOld() []byte {
	return marshalutil.New(1 + marshalutil.Uint16Size).
		WriteByte(byte(ReferenceUnlockBlockType)).
		WriteUint16(r.referenceUnlockBlockInner.ReferencedIndex).
		Bytes()
}

// String returns a human readable version of the UnlockBlock.
func (r *ReferenceUnlockBlock) String() string {
	return stringify.Struct("ReferenceUnlockBlock",
		stringify.StructField("referencedIndex", int(r.referenceUnlockBlockInner.ReferencedIndex)),
	)
}

// code contract (make sure the type implements all required methods)
var _ UnlockBlock = &ReferenceUnlockBlock{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AliasUnlockBlock /////////////////////////////////////////////////////////////////////////////////////////

// AliasUnlockBlock defines an UnlockBlock which contains an index of corresponding AliasOutput.
type AliasUnlockBlock struct {
	aliasUnlockBlockInner `serix:"0"`
}
type aliasUnlockBlockInner struct {
	ReferencedIndex uint16 `serix:"0"`
}

// NewAliasUnlockBlock is the constructor for AliasUnlockBlocks.
func NewAliasUnlockBlock(chainInputIndex uint16) *AliasUnlockBlock {
	return &AliasUnlockBlock{
		aliasUnlockBlockInner{
			ReferencedIndex: chainInputIndex,
		},
	}
}

// AliasUnlockBlockFromBytes unmarshals a AliasUnlockBlock from a sequence of bytes.
func AliasUnlockBlockFromBytesOld(bytes []byte) (unlockBlock *AliasUnlockBlock, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if unlockBlock, err = AliasUnlockBlockFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse AliasUnlockBlock from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// AliasUnlockBlockFromBytes unmarshals a AliasUnlockBlock from a sequence of bytes.
func AliasUnlockBlockFromBytes(bytes []byte) (unlockBlock *AliasUnlockBlock, consumedBytes int, err error) {
	_, err = serix.DefaultAPI.Decode(context.Background(), bytes, unlockBlock, serix.WithValidation())
	if err != nil {
		return nil, consumedBytes, err
	}
	return
}

// AliasUnlockBlockFromMarshalUtil unmarshals a AliasUnlockBlock using a MarshalUtil (for easier unmarshaling).
func AliasUnlockBlockFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (unlockBlock *AliasUnlockBlock, err error) {
	unlockBlockType, err := marshalUtil.ReadByte()
	if err != nil {
		err = errors.Errorf("failed to parse UnlockBlockType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if UnlockBlockType(unlockBlockType) != AliasUnlockBlockType {
		err = errors.Errorf("invalid UnlockBlockType (%X): %w", unlockBlockType, cerrors.ErrParseBytesFailed)
		return
	}

	unlockBlock = &AliasUnlockBlock{}
	if unlockBlock.ReferencedIndex, err = marshalUtil.ReadUint16(); err != nil {
		err = errors.Errorf("failed to parse ReferencedIndex (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	return
}

// AliasInputIndex returns the index of the input, the AliasOutput which contains AliasAddress.
func (r *AliasUnlockBlock) AliasInputIndex() uint16 {
	return r.ReferencedIndex
}

// Type returns the UnlockBlockType of the UnlockBlock.
func (r *AliasUnlockBlock) Type() UnlockBlockType {
	return AliasUnlockBlockType
}

// Bytes returns a marshaled version of the Address.
func (r *AliasUnlockBlock) Bytes() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), r, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		return nil
	}
	return objBytes
}

// Bytes returns a marshaled version of the UnlockBlock.
func (r *AliasUnlockBlock) BytesOld() []byte {
	return marshalutil.New(1 + marshalutil.Uint16Size).
		WriteByte(byte(AliasUnlockBlockType)).
		WriteUint16(r.ReferencedIndex).
		Bytes()
}

// String returns a human readable version of the UnlockBlock.
func (r *AliasUnlockBlock) String() string {
	return stringify.Struct("AliasUnlockBlock",
		stringify.StructField("ReferencedIndex", int(r.ReferencedIndex)),
	)
}

// code contract (make sure the type implements all required methods).
var _ UnlockBlock = &AliasUnlockBlock{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
