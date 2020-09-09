package ledgerstate

import (
	"encoding/binary"
	"fmt"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/mr-tron/base58"
)

// region OutputType ///////////////////////////////////////////////////////////////////////////////////////////////////

// OutputType represents the type of an output. Different output types can have different unlock rules and allow for
// some relatively basic smart contract logic.
type OutputType = uint8

const (
	// SigLockedSingleOutputType represents an  output holding vanilla IOTA tokens.
	SigLockedSingleOutputType OutputType = iota

	// SigLockedColoredOutputType represents an output that holds colored coins.
	SigLockedColoredOutputType
)

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
	UnlockValid(tx *Transaction, unlockBlock UnlockBlock) bool

	// Bytes returns a marshaled version of this Output.
	Bytes() []byte

	// make Outputs storable in the ObjectStorage.
	objectstorage.StorableObject

	// make Outputs implement a String method.
	fmt.Stringer
}

// OutputFromBytes unmarshals an Output object from a sequence of bytes.
// It either creates a new object or fills the optionally provided object with the parsed information.
func OutputFromBytes(bytes []byte, optionalTargetObject ...Output) (result Output, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseOutput(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ParseOutput unmarshals an Output using the given marshalUtil (for easier marshaling/unmarshaling).
func ParseOutput(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...Output) (result Output, err error) {
	outputType, err := marshalUtil.ReadByte()
	if err != nil {
		err = fmt.Errorf("error while parsing OutputType: %w", err)

		return
	}

	switch outputType {
	case SigLockedSingleOutputType:
		result = &SigLockedSingleOutput{}
	default:
		err = fmt.Errorf("unsupported OutputType `%X`", outputType)

		return
	}

	_, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parsedBytes int, parseErr error) {
		parsedBytes, parseErr = result.UnmarshalObjectStorageValue(data)

		return
	})

	return
}

// OutputFromStorageKey get's called when we restore a Output from the storage.
// In contrast to other database models, it unmarshals some information from the key so we simply store the key before
// it gets handed over to UnmarshalObjectStorageValue (by the ObjectStorage).
func OutputFromStorageKey(keyBytes []byte, optionalTargetObject ...Output) (result Output, consumedBytes int, err error) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		if len(keyBytes) < 1 {
			err = fmt.Errorf("error while parsing OutputType: %w", err)

			return
		}

		switch keyBytes[0] {
		case SigLockedSingleOutputType:
			result = &SigLockedSingleOutput{}
		default:
			err = fmt.Errorf("unsupported OutputType `%X`", keyBytes[0])

			return
		}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to OutputFromStorageKey")
	}

	// parse information
	ParseOutputID()
	marshalUtil := marshalutil.New(keyBytes)
	result.address, err = address.Parse(marshalUtil)
	if err != nil {
		return
	}
	result.transactionID, err = transaction.ParseID(marshalUtil)
	if err != nil {
		return
	}
	result.storageKey = marshalutil.New(keyBytes[:transaction.OutputIDLength]).Bytes(true)
	consumedBytes = marshalUtil.ReadOffset()

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
	balances.Set(IOTAColor, o.balance)

	return balances
}

// UnlockValid determines if the given Transaction and the corresponding UnlockBlock are allowed to spend the Output.
func (o *SigLockedSingleOutput) UnlockValid(tx *Transaction, unlockBlock UnlockBlock) bool {
	// TODO: IMPLEMENT ACTUAL SIGNATURE CHECKS
	return true
}

// Address returns the address that this output is associated to.
func (o *SigLockedSingleOutput) Address() Address {
	return o.address
}

// Bytes returns a marshaled version of this Output.
func (o *SigLockedSingleOutput) Bytes() []byte {
	return o.ObjectStorageValue()
}

// Update is disabled and panics if it ever gets called - it is required to match StorableObject interface.
func (o *SigLockedSingleOutput) Update(objectstorage.StorableObject) {
	panic("updates disabled for Outputs")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match
// the StorableObject interface.
func (o *SigLockedSingleOutput) ObjectStorageKey() []byte {
	return marshalutil.New().
		WriteByte(SigLockedSingleOutputType).
		WriteBytes(o.id.Bytes()).
		Bytes()
}

// ObjectStorageValue marshals the Output into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the ObjectStorage.
func (o *SigLockedSingleOutput) ObjectStorageValue() []byte {
	return marshalutil.New().
		WriteByte(SigLockedSingleOutputType).
		WriteUint64(o.balance).
		WriteBytes(o.address.Bytes()).
		Bytes()
}

// UnmarshalObjectStorageValue restores a Output from a serialized version in the ObjectStorage with parts of the object
// being stored in its key rather than the content of the database to reduce storage requirements.
func (o *SigLockedSingleOutput) UnmarshalObjectStorageValue(valueBytes []byte) (consumedBytes int, err error) {
	marshalUtil := marshalutil.New(valueBytes)

	if o.balance, err = marshalUtil.ReadUint64(); err != nil {
		return marshalUtil.ReadOffset(), fmt.Errorf("error while parsing balance of SigLockedSingleOutput: %w", err)
	}

	if o.address, err = ParseAddress(marshalUtil); err != nil {
		return marshalUtil.ReadOffset(), fmt.Errorf("error while parsing address of SigLockedSingleOutput: %w", err)
	}

	return marshalUtil.ReadOffset(), nil
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

// OutputIDFromBase58 creates an output id from a base58 encoded string.
func OutputIDFromBase58(base58String string) (outputID OutputID, err error) {
	// decode string
	bytes, err := base58.Decode(base58String)
	if err != nil {
		err = fmt.Errorf("failed to decode base58 encoded string '%s': %w", base58String, err)

		return
	}

	// sanitize input
	if len(bytes) != OutputIDLength {
		err = fmt.Errorf("base58 decoded OutputID '%s' does not match expected length of %d bytes", base58String, OutputIDLength)

		return
	}

	// copy bytes to result
	copy(outputID[:], bytes)

	return
}

// OutputIDFromBytes unmarshals an OutputID from a sequence of bytes.
func OutputIDFromBytes(bytes []byte) (result OutputID, consumedBytes int, err error) {
	// parse the bytes
	marshalUtil := marshalutil.New(bytes)
	idBytes, err := marshalUtil.ReadBytes(OutputIDLength)
	if err != nil {
		err = fmt.Errorf("failed to read bytes of OutputID: %w", err)

		return
	}
	copy(result[:], idBytes)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ParseOutputID is a wrapper for simplified unmarshaling of OutputIDs from a byte stream using the marshalUtil package.
func ParseOutputID(marshalUtil *marshalutil.MarshalUtil) (OutputID, error) {
	outputID, err := marshalUtil.Parse(func(data []byte) (interface{}, int, error) { return OutputIDFromBytes(data) })
	if err != nil {
		return OutputID{}, fmt.Errorf("error parsing OutputID: %w", err)
	}

	return outputID.(OutputID), nil
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

// String creates a human readable version base58 encoded of the OutputID.
func (o OutputID) String() string {
	return base58.Encode(o[:])
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
