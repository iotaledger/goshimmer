package tangle

import (
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/transaction"
)

// Consumer stores the information which transaction output was consumed by which transaction. We need this to be able
// to perform reverse lookups from transaction outputs to their corresponding consuming transactions.
type Consumer struct {
	objectstorage.StorableObjectFlags

	consumedInput transaction.OutputId
	transactionId transaction.Id

	storageKey []byte
}

// NewConsumer creates a Consumer object with the given information.
func NewConsumer(consumedInput transaction.OutputId, transactionId transaction.Id) *Consumer {
	return &Consumer{
		consumedInput: consumedInput,
		transactionId: transactionId,

		storageKey: marshalutil.New(ConsumerLength).
			WriteBytes(consumedInput.Bytes()).
			WriteBytes(transactionId.Bytes()).
			Bytes(),
	}
}

// ConsumerFromBytes unmarshals a Consumer from a sequence of bytes - it either creates a new object or fills the
// optionally provided one with the parsed information.
func ConsumerFromBytes(bytes []byte, optionalTargetObject ...*Consumer) (result *Consumer, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Consumer{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to ConsumerFromBytes")
	}

	// parse the bytes
	marshalUtil := marshalutil.New(bytes)
	if result.consumedInput, err = transaction.ParseOutputId(marshalUtil); err != nil {
		return
	}
	if result.transactionId, err = transaction.ParseId(marshalUtil); err != nil {
		return
	}
	result.storageKey = marshalutil.New(bytes[:ConsumerLength]).Bytes(true)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Parse is a wrapper for simplified unmarshaling of Consumers from a byte stream using the marshalUtil package.
func ParseConsumer(marshalUtil *marshalutil.MarshalUtil) (*Consumer, error) {
	if consumer, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return ConsumerFromBytes(data) }); err != nil {
		return nil, err
	} else {
		return consumer.(*Consumer), nil
	}
}

// ConsumerFromStorage gets called when we restore an Consumer from the storage - it parses the key bytes and
// returns the new object.
func ConsumerFromStorage(keyBytes []byte) objectstorage.StorableObject {
	result, err, _ := ConsumerFromBytes(keyBytes)
	if err != nil {
		panic(err)
	}

	return result
}

// ConsumedInput returns the OutputId of the Consumer.
func (consumer *Consumer) ConsumedInput() transaction.OutputId {
	return consumer.consumedInput
}

// TransactionId returns the transaction Id of this Consumer.
func (consumer *Consumer) TransactionId() transaction.Id {
	return consumer.transactionId
}

// Bytes marshals the Consumer into a sequence of bytes.
func (consumer *Consumer) Bytes() []byte {
	return consumer.ObjectStorageKey()
}

// String returns a human readable version of the Consumer.
func (consumer *Consumer) String() string {
	return stringify.Struct("Consumer",
		stringify.StructField("consumedInput", consumer.ConsumedInput()),
		stringify.StructField("transactionId", consumer.TransactionId()),
	)
}

// ObjectStorageKey returns the key that is used to store the object in the database.
func (consumer *Consumer) ObjectStorageKey() []byte {
	return consumer.storageKey
}

// ObjectStorageValue marshals the "content part" of an Consumer to a sequence of bytes. Since all of the information for
// this object are stored in its key, this method does nothing and is only required to conform with the interface.
func (consumer *Consumer) ObjectStorageValue() (data []byte) {
	return
}

// UnmarshalObjectStorageValue unmarshals the "content part" of a Consumer from a sequence of bytes. Since all of the information
// for this object are stored in its key, this method does nothing and is only required to conform with the interface.
func (consumer *Consumer) UnmarshalObjectStorageValue(data []byte) (err error) {
	return
}

// Update is disabled - updates are supposed to happen through the setters (if existing).
func (consumer *Consumer) Update(other objectstorage.StorableObject) {
	panic("update forbidden")
}

// Interface contract: make compiler warn if the interface is not implemented correctly.
var _ objectstorage.StorableObject = &Consumer{}

// ConsumerLength holds the length of a marshaled Consumer in bytes.
const ConsumerLength = transaction.OutputIdLength + transaction.IdLength
