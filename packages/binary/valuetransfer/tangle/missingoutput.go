package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/transaction"
)

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
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &MissingOutput{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to MissingOutputFromBytes")
	}

	// parse the bytes
	marshalUtil := marshalutil.New(bytes)
	if result.outputId, err = transaction.ParseOutputId(marshalUtil); err != nil {
		return
	}
	if result.missingSince, err = marshalUtil.ReadTime(); err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// MissingOutputFromStorage gets called when we restore a MissingOutput from the storage. The content will be
// unmarshaled by an external caller using the binary.MarshalBinary interface.
func MissingOutputFromStorage(keyBytes []byte) objectstorage.StorableObject {
	outputId, err, _ := transaction.OutputIdFromBytes(keyBytes)
	if err != nil {
		panic(err)
	}

	return &MissingOutput{
		outputId: outputId,
	}
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
	marshalUtil := marshalutil.New()

	marshalUtil.WriteBytes(missingOutput.outputId.Bytes())
	marshalUtil.WriteTime(missingOutput.missingSince)

	return marshalUtil.Bytes()
}

// GetStorageKey returns the key that is used to store the object in the object storage.
func (missingOutput *MissingOutput) GetStorageKey() []byte {
	return missingOutput.outputId.Bytes()
}

// Update is disabled and panics if it ever gets called - updates are supposed to happen through the setters.
func (missingOutput *MissingOutput) Update(other objectstorage.StorableObject) {
	panic("implement me")
}

// MarshalBinary returns a bytes representation of the Transaction by implementing the encoding.BinaryMarshaler
// interface.
func (missingOutput *MissingOutput) MarshalBinary() (data []byte, err error) {
	return missingOutput.Bytes(), nil
}

// UnmarshalBinary restores the values of a MissingOutput from a sequence of bytes using the  encoding.BinaryUnmarshaler
// interface.
func (missingOutput *MissingOutput) UnmarshalBinary(data []byte) (err error) {
	_, err, _ = MissingOutputFromBytes(data, missingOutput)

	return
}
