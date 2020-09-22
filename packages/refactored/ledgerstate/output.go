package ledgerstate

import (
	"encoding/binary"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/mr-tron/base58"
	"github.com/pkg/errors"
	"golang.org/x/xerrors"
)

// region OutputType ///////////////////////////////////////////////////////////////////////////////////////////////////

// OutputType represents the type of an output. Different output types can have different unlock rules and allow for
// some relatively basic smart contract logic.
type OutputType uint8

const (
	// SigLockedSingleOutputType represents an  output holding vanilla IOTA tokens.
	SigLockedSingleOutputType OutputType = iota

	// SigLockedColoredOutputType represents an output that holds colored coins.
	SigLockedColoredOutputType
)

// String returns a human readable representation of the OutputType.
func (o OutputType) String() string {
	return [...]string{
		"SigLockedSingleOutputType",
		"SigLockedColoredOutputType",
	}[o]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputID /////////////////////////////////////////////////////////////////////////////////////////////////////

// OutputIDLength contains the amount of bytes that a marshaled version of the OutputID contains.
const OutputIDLength = TransactionIDLength + marshalutil.UINT16_SIZE

// OutputID is the data type that represents the identifier for a SigLockedSingleOutput.
type OutputID [OutputIDLength]byte

// NewOutputID is the constructor for the OutputID type.
func NewOutputID(transactionID TransactionID, outputIndex uint16) (outputID OutputID) {
	copy(outputID[:TransactionIDLength], transactionID.Bytes())
	binary.LittleEndian.PutUint16(outputID[TransactionIDLength:], outputIndex)

	return
}

// OutputIDFromBytes unmarshals an OutputID from a sequence of bytes.
func OutputIDFromBytes(bytes []byte) (outputID OutputID, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if outputID, err = OutputIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse OutputID: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// OutputIDFromBase58 creates an OutputID from a base58 encoded string.
func OutputIDFromBase58(base58String string) (outputID OutputID, err error) {
	bytes, err := base58.Decode(base58String)
	if err != nil {
		err = xerrors.Errorf("error while decoding base58 encoded OutputID (%v): %w", err, ErrBase58DecodeFailed)
		return
	}

	if outputID, _, err = OutputIDFromBytes(bytes); err != nil {
		err = xerrors.Errorf("failed to parse OutputID: %w", err)
		return
	}

	return
}

// OutputIDFromMarshalUtil unmarshals an OutputID using a MarshalUtil (for easier unmarshaling).
func OutputIDFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (outputID OutputID, err error) {
	outputIDBytes, err := marshalUtil.ReadBytes(OutputIDLength)
	if err != nil {
		err = errors.Wrap(err, "failed to read bytes to parse OutputID")
	}
	copy(outputID[:], outputIDBytes)

	return
}

// TransactionID returns the TransactionID part of an OutputID.
func (o OutputID) TransactionID() (transactionID TransactionID) {
	copy(transactionID[:], o[:TransactionIDLength])

	return
}

// OutputIndex returns the output index part of an OutputID.
func (o OutputID) OutputIndex() uint16 {
	return binary.LittleEndian.Uint16(o[TransactionIDLength:])
}

// Bytes marshals the OutputID into a sequence of bytes.
func (o OutputID) Bytes() []byte {
	return o[:]
}

// Base58 returns a base58 encoded version of the OutputID.
func (o OutputID) Base58() string {
	return base58.Encode(o[:])
}

// String creates a human readable version base58 encoded of the OutputID.
func (o OutputID) String() string {
	return stringify.Struct("OutputID",
		stringify.StructField("transactionID", o.TransactionID()),
		stringify.StructField("outputIndex", o.OutputIndex()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Output ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Output is a generic interface for the different types of Outputs (with different unlock behaviors).
type Output interface {
	// ID returns the identifier of the output that is used to address the Output in the UTXODAG.
	ID() OutputID

	// SetID allows to set the identifier of the Output. We offer a setter for this property since Outputs that are
	// created to become part of a transaction usually do not have an identifier, yet as their identifier depends on
	// the TransactionID that is only determinable after the Transaction has been fully constructed. The ID is therefore
	// only accessed when the Output is supposed to be persisted.
	SetID(outputID OutputID)

	// Type returns the type of the Output, which allows us to generically handle Outputs of different types.
	Type() OutputType

	// Balances returns the funds that are associated with this Output.
	Balances() *ColoredBalances

	// UnlockValid determines if the given Transaction and the corresponding UnlockBlock are allowed to spend the
	// Output.
	UnlockValid(tx *Transaction, unlockBlock UnlockBlock) (bool, error)

	// Bytes returns a marshaled version of this Output.
	Bytes() []byte

	// String returns a human readable version of the Output for debug purposes.
	String() string

	// make Outputs storable in the ObjectStorage.
	objectstorage.StorableObject
}

// OutputFromBytes unmarshals an Output from a sequence of bytes.
func OutputFromBytes(bytes []byte) (result Output, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = OutputFromMarshalUtil(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// OutputFromMarshalUtil unmarshals an Output using a MarshalUtil (for easier unmarshaling).
func OutputFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (result Output, err error) {
	outputType, err := marshalUtil.ReadByte()
	if err != nil {
		err = errors.Wrap(err, "error while parsing OutputType")

		return
	}
	marshalUtil.ReadSeek(-1)

	switch OutputType(outputType) {
	case SigLockedSingleOutputType:
		result, err = SigLockedSingleOutputFromMarshalUtil(marshalUtil)
	case SigLockedColoredOutputType:
		// TODO: PARSE OUTPUT
	default:
		err = errors.Errorf("unsupported OutputType `%X`", outputType)
	}

	return
}

// OutputFromObjectStorage restores an Output that was stored in the ObjectStorage.
func OutputFromObjectStorage(key []byte, data []byte) (result Output, err error) {
	result, err = OutputFromMarshalUtil(marshalutil.New(data))
	if err != nil {
		return
	}

	outputID, err := OutputIDFromMarshalUtil(marshalutil.New(key))
	if err != nil {
		return
	}
	result.SetID(outputID)

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SigLockedSingleOutput ////////////////////////////////////////////////////////////////////////////////////////

// SigLockedSingleOutput is an Output that holds exactly one uncolored balance and that can be unlocked by providing a
// signature for the given address.
type SigLockedSingleOutput struct {
	id      OutputID
	address Address
	balance uint64

	objectstorage.StorableObjectFlags
}

// SigLockedSingleOutputFromBytes unmarshals a SigLockedSingleOutput from a sequence of bytes.
func SigLockedSingleOutputFromBytes(bytes []byte) (result *SigLockedSingleOutput, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = SigLockedSingleOutputFromMarshalUtil(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// SigLockedSingleOutputFromMarshalUtil unmarshals a SigLockedSingleOutput using a MarshalUtil (for easier unmarshaling).
func SigLockedSingleOutputFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (result *SigLockedSingleOutput, err error) {
	outputType, err := marshalUtil.ReadByte()
	if err != nil {
		err = errors.Wrap(err, "error parsing OutputType")
		return
	}
	if OutputType(outputType) != SigLockedSingleOutputType {
		err = errors.Errorf("invalid OutputType `%X`", outputType)
		return
	}

	result = &SigLockedSingleOutput{}
	if result.balance, err = marshalUtil.ReadUint64(); err != nil {
		err = errors.Wrap(err, "error parsing balance")
		return
	}
	if result.address, err = AddressFromMarshalUtil(marshalUtil); err != nil {
		return
	}

	return
}

// ID returns the identifier of the output that is used to address the Output in the UTXODAG.
func (o *SigLockedSingleOutput) ID() OutputID {
	return o.id
}

// SetID allows to set the identifier of the Output. We offer a setter for this property since Outputs that are
// created to become part of a transaction usually do not have an identifier, yet as their identifier depends on
// the TransactionID that is only determinable after the Transaction has been fully constructed. The ID is therefore
// only accessed when the Output is supposed to be persisted by the node.
func (o *SigLockedSingleOutput) SetID(outputID OutputID) {
	o.id = outputID
}

// Type returns the type of the Output, which allows us to generically handle Outputs of different types.
func (o *SigLockedSingleOutput) Type() OutputType {
	return SigLockedSingleOutputType
}

// Balances returns the funds that are associated with this Output.
func (o *SigLockedSingleOutput) Balances() *ColoredBalances {
	balances := NewColoredBalances()
	balances.Set(ColorIOTA, o.balance)

	return balances
}

// UnlockValid determines if the given Transaction and the corresponding UnlockBlock are allowed to spend the Output.
func (o *SigLockedSingleOutput) UnlockValid(tx *Transaction, unlockBlock UnlockBlock) (bool, error) {
	signatureUnlockBlock, correctType := unlockBlock.(*SignatureUnlockBlock)
	if !correctType {
		return false, errors.New("UnlockBlock does not match OutputType")
	}

	return signatureUnlockBlock.SignatureValid(o.address, tx.UnsignedBytes())
}

// Address returns the Address that this output is associated to.
func (o *SigLockedSingleOutput) Address() Address {
	return o.address
}

// Bytes returns a marshaled version of this Output.
func (o *SigLockedSingleOutput) Bytes() []byte {
	return o.ObjectStorageValue()
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (o *SigLockedSingleOutput) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match
// the StorableObject interface.
func (o *SigLockedSingleOutput) ObjectStorageKey() []byte {
	return o.id.Bytes()
}

// ObjectStorageValue marshals the Output into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the ObjectStorage.
func (o *SigLockedSingleOutput) ObjectStorageValue() []byte {
	return marshalutil.New().
		WriteByte(byte(SigLockedSingleOutputType)).
		WriteUint64(o.balance).
		WriteBytes(o.address.Bytes()).
		Bytes()
}

// String returns a human readable version of this Output.
func (o *SigLockedSingleOutput) String() string {
	return stringify.Struct("SigLockedSingleOutput",
		stringify.StructField("id", o.id),
		stringify.StructField("address", o.address),
		stringify.StructField("balance", o.balance),
	)
}

// code contract (make sure the type implements all required methods)
var _ Output = &SigLockedSingleOutput{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SigLockedColoredOutput ////////////////////////////////////////////////////////////////////////////////////////

// SigLockedColoredOutput is an Output that holds colored balances and that can be unlocked by providing a signature for
// the given address.
type SigLockedColoredOutput struct {
	id       OutputID
	address  Address
	balances *ColoredBalances

	objectstorage.StorableObjectFlags
}

// SigLockedColoredOutputFromBytes unmarshals a SigLockedColoredOutput from a sequence of bytes.
func SigLockedColoredOutputFromBytes(bytes []byte) (result *SigLockedColoredOutput, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = SigLockedColoredOutputFromMarshalUtil(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// SigLockedColoredOutputFromMarshalUtil unmarshals a SigLockedColoredOutput using a MarshalUtil (for easier unmarshaling).
func SigLockedColoredOutputFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (result *SigLockedColoredOutput, err error) {
	outputType, err := marshalUtil.ReadByte()
	if err != nil {
		err = errors.Wrap(err, "error parsing OutputType")
		return
	}
	if OutputType(outputType) != SigLockedColoredOutputType {
		err = errors.Errorf("invalid OutputType `%X`", outputType)
		return
	}

	result = &SigLockedColoredOutput{}
	if result.balances, err = ColoredBalancesFromMarshalUtil(marshalUtil); err != nil {
		return
	}
	if result.address, err = AddressFromMarshalUtil(marshalUtil); err != nil {
		return
	}

	return
}

// ID returns the identifier of the output that is used to address the Output in the UTXODAG.
func (o *SigLockedColoredOutput) ID() OutputID {
	return o.id
}

// SetID allows to set the identifier of the Output. We offer a setter for this property since Outputs that are
// created to become part of a transaction usually do not have an identifier, yet as their identifier depends on
// the TransactionID that is only determinable after the Transaction has been fully constructed. The ID is therefore
// only accessed when the Output is supposed to be persisted by the node.
func (o *SigLockedColoredOutput) SetID(outputID OutputID) {
	o.id = outputID
}

// Type returns the type of the Output, which allows us to generically handle Outputs of different types.
func (o *SigLockedColoredOutput) Type() OutputType {
	return SigLockedColoredOutputType
}

// Balances returns the funds that are associated with this Output.
func (o *SigLockedColoredOutput) Balances() *ColoredBalances {
	return o.balances
}

// UnlockValid determines if the given Transaction and the corresponding UnlockBlock are allowed to spend the Output.
func (o *SigLockedColoredOutput) UnlockValid(tx *Transaction, unlockBlock UnlockBlock) (bool, error) {
	signatureUnlockBlock, correctType := unlockBlock.(*SignatureUnlockBlock)
	if !correctType {
		return false, errors.New("UnlockBlock does not match OutputType")
	}

	return signatureUnlockBlock.SignatureValid(o.address, tx.UnsignedBytes())
}

// Address returns the Address that this output is associated to.
func (o *SigLockedColoredOutput) Address() Address {
	return o.address
}

// Bytes returns a marshaled version of this Output.
func (o *SigLockedColoredOutput) Bytes() []byte {
	return o.ObjectStorageValue()
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (o *SigLockedColoredOutput) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match
// the StorableObject interface.
func (o *SigLockedColoredOutput) ObjectStorageKey() []byte {
	return o.id.Bytes()
}

// ObjectStorageValue marshals the Output into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the ObjectStorage.
func (o *SigLockedColoredOutput) ObjectStorageValue() []byte {
	return marshalutil.New().
		WriteByte(byte(SigLockedColoredOutputType)).
		WriteBytes(o.balances.Bytes()).
		WriteBytes(o.address.Bytes()).
		Bytes()
}

// String returns a human readable version of this Output.
func (o *SigLockedColoredOutput) String() string {
	return stringify.Struct("SigLockedColoredOutput",
		stringify.StructField("id", o.id),
		stringify.StructField("address", o.address),
		stringify.StructField("balance", o.balances),
	)
}

// code contract (make sure the type implements all required methods)
var _ Output = &SigLockedColoredOutput{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedOutput /////////////////////////////////////////////////////////////////////////////////////////////////

// CachedOutput is a wrapper for the generic CachedObject returned by the objectstorage, that overrides the accessor
// methods, with a type-casted one.
type CachedOutput struct {
	objectstorage.CachedObject
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (cachedOutput *CachedOutput) Unwrap() Output {
	untypedObject := cachedOutput.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(Output)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (cachedOutput *CachedOutput) Consume(consumer func(output Output)) (consumed bool) {
	return cachedOutput.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(Output))
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
