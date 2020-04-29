package utxodag

import (
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

// ConsumerPartitionKeys defines the "layout" of the key. This enables prefix iterations in the objectstorage.
var ConsumerPartitionKeys = objectstorage.PartitionKey([]int{address.Length, transaction.IDLength, transaction.IDLength}...)

// Consumer stores the information which transaction output was consumed by which transaction. We need this to be able
// to perform reverse lookups from transaction outputs to their corresponding consuming transactions.
type Consumer struct {
	objectstorage.StorableObjectFlags

	consumedInput transaction.OutputID
	transactionID transaction.ID

	storageKey []byte
}

// NewConsumer creates a Consumer object with the given information.
func NewConsumer(consumedInput transaction.OutputID, transactionID transaction.ID) *Consumer {
	return &Consumer{
		consumedInput: consumedInput,
		transactionID: transactionID,

		storageKey: marshalutil.New(ConsumerLength).
			WriteBytes(consumedInput.Bytes()).
			WriteBytes(transactionID.Bytes()).
			Bytes(),
	}
}

// ConsumerFromBytes unmarshals a Consumer from a sequence of bytes - it either creates a new object or fills the
// optionally provided one with the parsed information.
func ConsumerFromBytes(bytes []byte, optionalTargetObject ...*Consumer) (result *Consumer, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseConsumer(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ParseConsumer unmarshals a Consumer using the given marshalUtil (for easier marshaling/unmarshaling).
func ParseConsumer(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*Consumer) (result *Consumer, err error) {
	parsedObject, parseErr := marshalUtil.Parse(func(data []byte) (interface{}, int, error) {
		return ConsumerFromStorageKey(data, optionalTargetObject...)
	})
	if parseErr != nil {
		err = parseErr

		return
	}

	result = parsedObject.(*Consumer)

	if _, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parsedBytes int, parseErr error) {
		parsedBytes, parseErr = result.UnmarshalObjectStorageValue(data)

		return
	}); err != nil {
		return
	}

	return
}

// ConsumerFromStorageKey is a factory method that creates a new Consumer instance from a storage key of the
// objectstorage. It is used by the objectstorage, to create new instances of this entity.
func ConsumerFromStorageKey(key []byte, optionalTargetObject ...*Consumer) (result *Consumer, consumedBytes int, err error) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Consumer{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to ConsumerFromStorageKey")
	}

	// parse the properties that are stored in the key
	marshalUtil := marshalutil.New(key)
	if result.consumedInput, err = transaction.ParseOutputID(marshalUtil); err != nil {
		return
	}
	if result.transactionID, err = transaction.ParseID(marshalUtil); err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()
	result.storageKey = marshalutil.New(key[:consumedBytes]).Bytes(true)

	return
}

// ConsumedInput returns the OutputID of the Consumer.
func (consumer *Consumer) ConsumedInput() transaction.OutputID {
	return consumer.consumedInput
}

// TransactionID returns the transaction ID of this Consumer.
func (consumer *Consumer) TransactionID() transaction.ID {
	return consumer.transactionID
}

// Bytes marshals the Consumer into a sequence of bytes.
func (consumer *Consumer) Bytes() []byte {
	return consumer.ObjectStorageKey()
}

// String returns a human readable version of the Consumer.
func (consumer *Consumer) String() string {
	return stringify.Struct("Consumer",
		stringify.StructField("consumedInput", consumer.ConsumedInput()),
		stringify.StructField("transactionId", consumer.TransactionID()),
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
func (consumer *Consumer) UnmarshalObjectStorageValue(data []byte) (consumedBytes int, err error) {
	return
}

// Update is disabled - updates are supposed to happen through the setters (if existing).
func (consumer *Consumer) Update(other objectstorage.StorableObject) {
	panic("update forbidden")
}

// Interface contract: make compiler warn if the interface is not implemented correctly.
var _ objectstorage.StorableObject = &Consumer{}

// ConsumerLength holds the length of a marshaled Consumer in bytes.
const ConsumerLength = transaction.OutputIDLength + transaction.IDLength

// region CachedConsumer /////////////////////////////////////////////////////////////////////////////////////////////////

// CachedConsumer is a wrapper for the generic CachedObject returned by the objectstorage, that overrides the accessor
// methods, with a type-casted one.
type CachedConsumer struct {
	objectstorage.CachedObject
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (cachedConsumer *CachedConsumer) Unwrap() *Consumer {
	untypedObject := cachedConsumer.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*Consumer)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (cachedConsumer *CachedConsumer) Consume(consumer func(consumer *Consumer)) (consumed bool) {
	return cachedConsumer.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Consumer))
	})
}

// CachedConsumers represents a collection of CachedConsumers.
type CachedConsumers []*CachedConsumer

// Consume iterates over the CachedObjects, unwraps them and passes a type-casted version to the consumer (if the object
// is not empty - it exists). It automatically releases the object when the consumer finishes. It returns true, if at
// least one object was consumed.
func (cachedConsumers CachedConsumers) Consume(consumer func(consumer *Consumer)) (consumed bool) {
	for _, cachedConsumer := range cachedConsumers {
		consumed = cachedConsumer.Consume(func(output *Consumer) {
			consumer(output)
		}) || consumed
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
