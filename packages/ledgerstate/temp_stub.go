package ledgerstate

import (
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

// region TEMPORARY DEFINITIONS TO PREVENT ERRORS DUE TO UNMERGED MODELS ///////////////////////////////////////////////

// TransactionIDLength contains the amount of bytes that a marshaled version of the ID contains.
const TransactionIDLength = 32

// TransactionID is the type that represents the identifier of a Transaction.
type TransactionID [TransactionIDLength]byte

// TransactionIDFromBytes unmarshals a TransactionID from a sequence of bytes.
func TransactionIDFromBytes(bytes []byte) (result TransactionID, consumedBytes int, err error) {
	// parse the bytes
	marshalUtil := marshalutil.New(bytes)
	idBytes, idErr := marshalUtil.ReadBytes(TransactionIDLength)
	if idErr != nil {
		err = idErr

		return
	}
	copy(result[:], idBytes)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// TransactionIDFromMarshalUtil is a wrapper for simplified unmarshaling of TransactionIDs from a byte stream using the
// marshalUtil package.
func TransactionIDFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (TransactionID, error) {
	id, err := marshalUtil.Parse(func(data []byte) (interface{}, int, error) { return TransactionIDFromBytes(data) })
	if err != nil {
		return TransactionID{}, err
	}

	return id.(TransactionID), nil
}

// Bytes marshals the ID into a sequence of bytes.
func (i TransactionID) Bytes() []byte {
	return i[:]
}

// Transaction represents a value transfer.
type Transaction struct{}

// UnsignedBytes returns the unsigned bytes of the Transaction.
func (t *Transaction) UnsignedBytes() []byte {
	return nil
}

// BranchIDLength contains the amount of bytes that a marshaled version of the BranchID contains.
const BranchIDLength = 32

// BranchID is the data type that represents the identifier of a Branch.
type BranchID [BranchIDLength]byte

// BranchIDFromMarshalUtil unmarshals a BranchID using a MarshalUtil (for easier unmarshaling).
func BranchIDFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (branchID BranchID, err error) {
	branchIDBytes, err := marshalUtil.ReadBytes(BranchIDLength)
	if err != nil {
		err = xerrors.Errorf("failed to parse BranchID (%v): %w", err, ErrParseBytesFailed)
		return
	}
	copy(branchID[:], branchIDBytes)

	return
}

// Bytes returns a marshaled version of this BranchID.
func (b BranchID) Bytes() []byte {
	return b[:]
}

const (
	// SignatureUnlockBlockType represents the type of a SignatureUnlockBlock.
	SignatureUnlockBlockType UnlockBlockType = iota

	// ReferenceUnlockBlockType represents the type of a ReferenceUnlockBlock.
	ReferenceUnlockBlockType
)

// UnlockBlockType represents the type of the UnlockBlock. Different types of UnlockBlocks can unlock different types of
// Outputs.
type UnlockBlockType uint8

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

// SignatureUnlockBlock represents an UnlockBlock that contains a Signature for an Address.
type SignatureUnlockBlock struct {
	signature Signature
}

// AddressSignatureValid returns true if this UnlockBlock correctly signs the given Address.
func (s *SignatureUnlockBlock) AddressSignatureValid(address Address, signedData []byte) (bool, error) {
	return s.signature.AddressSignatureValid(address, signedData), nil
}

// Type returns the UnlockBlockType of this UnlockBlock.
func (s *SignatureUnlockBlock) Type() UnlockBlockType {
	return SignatureUnlockBlockType
}

// Bytes returns a marshaled version of this UnlockBlock.
func (s *SignatureUnlockBlock) Bytes() []byte {
	return byteutils.ConcatBytes([]byte{byte(SignatureUnlockBlockType)}, s.signature.Bytes())
}

// String returns a human readable version of this UnlockBlock.
func (s *SignatureUnlockBlock) String() string {
	return stringify.Struct("SignatureUnlockBlock",
		stringify.StructField("signature", s.signature),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
