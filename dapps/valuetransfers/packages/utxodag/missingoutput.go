package utxodag

import (
	"time"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

// MissingOutputKeyPartitions defines the "layout" of the key. This enables prefix iterations in the objectstorage.
var MissingOutputKeyPartitions = objectstorage.PartitionKey([]int{address.Length, transaction.IDLength}...)

// MissingOutput represents an Output that was referenced by a Transaction, but that is missing in our object storage.
type MissingOutput struct {
	objectstorage.StorableObjectFlags

	outputID     transaction.OutputID
	missingSince time.Time
}

// NewMissingOutput creates a new MissingOutput object, that .
func NewMissingOutput(outputID transaction.OutputID) *MissingOutput {
	return &MissingOutput{
		outputID:     outputID,
		missingSince: time.Now(),
	}
}

// MissingOutputFromBytes unmarshals a MissingOutput from a sequence of bytes - it either creates a new object or fills
// the optionally provided one with the parsed information.
func MissingOutputFromBytes(bytes []byte, optionalTargetObject ...*MissingOutput) (result *MissingOutput, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseMissingOutput(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ParseMissingOutput unmarshals a MissingOutput using the given marshalUtil (for easier marshaling/unmarshaling).
func ParseMissingOutput(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*MissingOutput) (result *MissingOutput, err error) {
	parsedObject, parseErr := marshalUtil.Parse(func(data []byte) (interface{}, int, error) {
		return MissingOutputFromStorageKey(data, optionalTargetObject...)
	})
	if parseErr != nil {
		err = parseErr

		return
	}

	result = parsedObject.(*MissingOutput)

	if _, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parsedBytes int, parseErr error) {
		parsedBytes, parseErr = result.UnmarshalObjectStorageValue(data)

		return
	}); err != nil {
		return
	}

	return
}

// MissingOutputFromStorageKey gets called when we restore a MissingOutput from the storage. The content will be
// unmarshaled by an external caller using the binary.ObjectStorageValue interface.
func MissingOutputFromStorageKey(key []byte, optionalTargetObject ...*MissingOutput) (result *MissingOutput, consumedBytes int, err error) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &MissingOutput{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to MissingOutputFromStorageKey")
	}

	// parse the properties that are stored in the key
	marshalUtil := marshalutil.New(key)
	if result.outputID, err = transaction.ParseOutputID(marshalUtil); err != nil {
		return
	}

	return
}

// ID returns the id of the Output that is missing.
func (missingOutput *MissingOutput) ID() transaction.OutputID {
	return missingOutput.outputID
}

// MissingSince returns the Time since the transaction was first reported as being missing.
func (missingOutput *MissingOutput) MissingSince() time.Time {
	return missingOutput.missingSince
}

// Bytes marshals the MissingOutput into a sequence of bytes.
func (missingOutput *MissingOutput) Bytes() []byte {
	return marshalutil.New(transaction.OutputIDLength + marshalutil.TIME_SIZE).
		WriteBytes(missingOutput.ObjectStorageKey()).
		WriteBytes(missingOutput.ObjectStorageValue()).
		Bytes()
}

// ObjectStorageKey returns the key that is used to store the object in the object storage.
func (missingOutput *MissingOutput) ObjectStorageKey() []byte {
	return missingOutput.outputID.Bytes()
}

// ObjectStorageValue returns a bytes representation of the Transaction by implementing the encoding.BinaryMarshaler
// interface.
func (missingOutput *MissingOutput) ObjectStorageValue() []byte {
	return marshalutil.New(marshalutil.TIME_SIZE).
		WriteTime(missingOutput.missingSince).
		Bytes()
}

// UnmarshalObjectStorageValue restores the values of a MissingOutput from a sequence of bytes using the  encoding.BinaryUnmarshaler
// interface.
func (missingOutput *MissingOutput) UnmarshalObjectStorageValue(data []byte) (consumedBytes int, err error) {
	marshalUtil := marshalutil.New(data)
	if missingOutput.missingSince, err = marshalUtil.ReadTime(); err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Update is disabled and panics if it ever gets called - updates are supposed to happen through the setters.
func (missingOutput *MissingOutput) Update(other objectstorage.StorableObject) {
	panic("implement me")
}

// Interface contract: make compiler warn if the interface is not implemented correctly.
var _ objectstorage.StorableObject = &MissingOutput{}
