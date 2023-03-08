package devnetvm

import (
	"context"
	"strconv"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/model"
	"github.com/iotaledger/hive.go/serializer/v2/serix"
	"github.com/iotaledger/hive.go/stringify"
)

func init() {
	err := serix.DefaultAPI.RegisterTypeSettings(AliasUnlockBlock{}, serix.TypeSettings{}.WithObjectType(uint8(new(AliasUnlockBlock).Type())))
	if err != nil {
		panic(errors.Wrap(err, "error registering AliasUnlockBlock type settings"))
	}
	err = serix.DefaultAPI.RegisterTypeSettings(ReferenceUnlockBlock{}, serix.TypeSettings{}.WithObjectType(uint8(new(ReferenceUnlockBlock).Type())))
	if err != nil {
		panic(errors.Wrap(err, "error registering ReferenceUnlockBlock type settings"))
	}
	err = serix.DefaultAPI.RegisterTypeSettings(SignatureUnlockBlock{}, serix.TypeSettings{}.WithObjectType(uint8(new(SignatureUnlockBlock).Type())))
	if err != nil {
		panic(errors.Wrap(err, "error registering SignatureUnlockBlock type settings"))
	}
	err = serix.DefaultAPI.RegisterTypeSettings(UnlockBlocks{}, serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsUint16).WithArrayRules(&serix.ArrayRules{
		// TODO: Avoid failing on duplicated unlock blocks. They seem to have been wrongly generated in the old snapshot.
		// ValidationMode: serializer.ArrayValidationModeNoDuplicates,
	}))
	if err != nil {
		panic(errors.Wrap(err, "error registering SignatureUnlockBlock type settings"))
	}
	err = serix.DefaultAPI.RegisterInterfaceObjects((*UnlockBlock)(nil), new(AliasUnlockBlock), new(ReferenceUnlockBlock), new(SignatureUnlockBlock))
	if err != nil {
		panic(errors.Wrap(err, "error registering UnlockBlock interface implementations"))
	}
}

// region UnlockBlockType. //////////////////////////////////////////////////////////////////////////////////////////////.
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
	Bytes() ([]byte, error)

	// String returns a human readable version of the UnlockBlock.
	String() string
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region UnlockBlocks /////////////////////////////////////////////////////////////////////////////////////////////////

// UnlockBlocks is slice of UnlockBlocks that offers additional methods for easier marshaling and unmarshaling.
type UnlockBlocks []UnlockBlock

// UnlockBlocksFromBytes unmarshals UnlockBlocks from a sequence of bytes.
func UnlockBlocksFromBytes(bytes []byte) (unlockBlocks UnlockBlocks, consumedBytes int, err error) {
	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), bytes, &unlockBlocks, serix.WithValidation())
	if err != nil {
		err = errors.Wrap(err, "failed to parse UnlockBlocks")
		return
	}

	return
}

// Bytes returns a marshaled version of the UnlockBlocks.
func (u UnlockBlocks) Bytes() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), u)
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// String returns a human readable version of the UnlockBlocks.
func (u UnlockBlocks) String() string {
	structBuilder := stringify.NewStructBuilder("UnlockBlocks")
	for i, unlockBlock := range u {
		structBuilder.AddField(stringify.NewStructField(strconv.Itoa(i), unlockBlock))
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SignatureUnlockBlock /////////////////////////////////////////////////////////////////////////////////////////

// SignatureUnlockBlock represents an UnlockBlock that contains a Signature for an Address.
type SignatureUnlockBlock struct {
	model.Immutable[SignatureUnlockBlock, *SignatureUnlockBlock, signatureUnlockBlockModel] `serix:"0"`
}
type signatureUnlockBlockModel struct {
	Signature Signature `serix:"0"`
}

// NewSignatureUnlockBlock is the constructor for SignatureUnlockBlock objects.
func NewSignatureUnlockBlock(signature Signature) *SignatureUnlockBlock {
	return model.NewImmutable[SignatureUnlockBlock](&signatureUnlockBlockModel{
		Signature: signature,
	})
}

// AddressSignatureValid returns true if the UnlockBlock correctly signs the given Address.
func (s *SignatureUnlockBlock) AddressSignatureValid(address Address, signedData []byte) bool {
	return s.Signature().AddressSignatureValid(address, signedData)
}

// Type returns the UnlockBlockType of the UnlockBlock.
func (s *SignatureUnlockBlock) Type() UnlockBlockType {
	return SignatureUnlockBlockType
}

// Signature return the signature itself.
func (s *SignatureUnlockBlock) Signature() Signature {
	return s.M.Signature
}

// code contract (make sure the type implements all required methods).
var _ UnlockBlock = &SignatureUnlockBlock{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ReferenceUnlockBlock /////////////////////////////////////////////////////////////////////////////////////////

// ReferenceUnlockBlock defines an UnlockBlock which references a previous UnlockBlock (which must not be another
// ReferenceUnlockBlock).
type ReferenceUnlockBlock struct {
	model.Immutable[ReferenceUnlockBlock, *ReferenceUnlockBlock, referenceUnlockBlockModel] `serix:"0"`
}
type referenceUnlockBlockModel struct {
	ReferencedIndex uint16 `serix:"0"`
}

// NewReferenceUnlockBlock is the constructor for ReferenceUnlockBlocks.
func NewReferenceUnlockBlock(referencedIndex uint16) *ReferenceUnlockBlock {
	return model.NewImmutable[ReferenceUnlockBlock](&referenceUnlockBlockModel{
		ReferencedIndex: referencedIndex,
	})
}

// ReferenceUnlockBlockFromBytes unmarshals a ReferenceUnlockBlock from a sequence of bytes.
func ReferenceUnlockBlockFromBytes(bytes []byte) (unlockBlock *ReferenceUnlockBlock, consumedBytes int, err error) {
	unlockBlock = new(ReferenceUnlockBlock)
	_, err = serix.DefaultAPI.Decode(context.Background(), bytes, unlockBlock, serix.WithValidation())
	if err != nil {
		return nil, consumedBytes, err
	}
	return
}

// ReferencedIndex returns the index of the referenced UnlockBlock.
func (r *ReferenceUnlockBlock) ReferencedIndex() uint16 {
	return r.M.ReferencedIndex
}

// Type returns the UnlockBlockType of the UnlockBlock.
func (r *ReferenceUnlockBlock) Type() UnlockBlockType {
	return ReferenceUnlockBlockType
}

// code contract (make sure the type implements all required methods).
var _ UnlockBlock = &ReferenceUnlockBlock{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AliasUnlockBlock /////////////////////////////////////////////////////////////////////////////////////////////

// AliasUnlockBlock defines an UnlockBlock which contains an index of corresponding AliasOutput.
type AliasUnlockBlock struct {
	model.Immutable[AliasUnlockBlock, *AliasUnlockBlock, aliasUnlockBlockModel] `serix:"0"`
}
type aliasUnlockBlockModel struct {
	ReferencedIndex uint16 `serix:"0"`
}

// NewAliasUnlockBlock is the constructor for AliasUnlockBlocks.
func NewAliasUnlockBlock(chainInputIndex uint16) *AliasUnlockBlock {
	return model.NewImmutable[AliasUnlockBlock](&aliasUnlockBlockModel{
		ReferencedIndex: chainInputIndex,
	})
}

// AliasInputIndex returns the index of the input, the AliasOutput which contains AliasAddress.
func (r *AliasUnlockBlock) AliasInputIndex() uint16 {
	return r.M.ReferencedIndex
}

// Type returns the UnlockBlockType of the UnlockBlock.
func (r *AliasUnlockBlock) Type() UnlockBlockType {
	return AliasUnlockBlockType
}

// code contract (make sure the type implements all required methods).
var _ UnlockBlock = &AliasUnlockBlock{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
