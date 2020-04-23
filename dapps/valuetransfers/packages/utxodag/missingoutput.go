package utxodag

import (
	"time"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

var MissingOutputKeyPartitions = objectstorage.PartitionKey([]int{address.Length, transaction.IdLength}...)

// MissingPayload represents an Output that was referenced by a Transaction, but that is missing in our object storage.
type MissingOutput struct {
	objectstorage.StorableObjectFlags

	outputId     transaction.OutputId
	missingSince time.Time
}

// NewMissingOutput creates a new MissingOutput object, that .
func NewMissingOutput(outputId transaction.OutputId) *MissingOutput {
	return &MissingOutput{
		outputId:     outputId,
		missingSince: time.Now(),
	}
}

// MissingOutputFromBytes unmarshals a MissingOutput from a sequence of bytes - it either creates a new object or fills
// the optionally provided one with the parsed information.
func MissingOutputFromBytes(bytes []byte, optionalTargetObject ...*MissingOutput) (result *MissingOutput, err error, consumedBytes int) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseMissingOutput(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func ParseMissingOutput(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*MissingOutput) (result *MissingOutput, err error) {
	if parsedObject, parseErr := marshalUtil.Parse(func(data []byte) (interface{}, error, int) {
		return MissingOutputFromStorageKey(data, optionalTargetObject...)
	}); parseErr != nil {
		err = parseErr

		return
	} else {
		result = parsedObject.(*MissingOutput)
	}

	if _, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parseErr error, parsedBytes int) {
		parseErr, parsedBytes = result.UnmarshalObjectStorageValue(data)

		return
	}); err != nil {
		return
	}

	return
}

// MissingOutputFromStorageKey gets called when we restore a MissingOutput from the storage. The content will be
// unmarshaled by an external caller using the binary.ObjectStorageValue interface.
func MissingOutputFromStorageKey(key []byte, optionalTargetObject ...*MissingOutput) (result *MissingOutput, err error, consumedBytes int) {
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
	if result.outputId, err = transaction.ParseOutputId(marshalUtil); err != nil {
		return
	}

	return
}

// Id returns the id of the Output that is missing.
func (missingOutput *MissingOutput) Id() transaction.OutputId {
	return missingOutput.outputId
}

// MissingSince returns the Time since the transaction was first reported as being missing.
func (missingOutput *MissingOutput) MissingSince() time.Time {
	return missingOutput.missingSince
}

// Bytes marshals the MissingOutput into a sequence of bytes.
func (missingOutput *MissingOutput) Bytes() []byte {
	return marshalutil.New(transaction.OutputIdLength + marshalutil.TIME_SIZE).
		WriteBytes(missingOutput.ObjectStorageKey()).
		WriteBytes(missingOutput.ObjectStorageValue()).
		Bytes()
}

// ObjectStorageKey returns the key that is used to store the object in the object storage.
func (missingOutput *MissingOutput) ObjectStorageKey() []byte {
	return missingOutput.outputId.Bytes()
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
func (missingOutput *MissingOutput) UnmarshalObjectStorageValue(data []byte) (err error, consumedBytes int) {
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
