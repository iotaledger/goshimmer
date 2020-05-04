package utxodag

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

// OutputKeyPartitions defines the "layout" of the key. This enables prefix iterations in the objectstorage.
var OutputKeyPartitions = objectstorage.PartitionKey([]int{address.Length, transaction.IDLength}...)

// Output represents the output of a Transaction and contains the balances and the identifiers for this output.
type Output struct {
	address            address.Address
	transactionID      transaction.ID
	branchID           branchmanager.BranchID
	solid              bool
	solidificationTime time.Time
	firstConsumer      transaction.ID
	consumerCount      int
	balances           []*balance.Balance

	branchIDMutex           sync.RWMutex
	solidMutex              sync.RWMutex
	solidificationTimeMutex sync.RWMutex
	consumerMutex           sync.RWMutex

	objectstorage.StorableObjectFlags
	storageKey []byte
}

// NewOutput creates an Output that contains the balances and identifiers of a Transaction.
func NewOutput(address address.Address, transactionID transaction.ID, branchID branchmanager.BranchID, balances []*balance.Balance) *Output {
	return &Output{
		address:            address,
		transactionID:      transactionID,
		branchID:           branchID,
		solid:              false,
		solidificationTime: time.Time{},
		balances:           balances,

		storageKey: marshalutil.New().WriteBytes(address.Bytes()).WriteBytes(transactionID.Bytes()).Bytes(),
	}
}

// OutputFromBytes unmarshals an Output object from a sequence of bytes.
// It either creates a new object or fills the optionally provided object with the parsed information.
func OutputFromBytes(bytes []byte, optionalTargetObject ...*Output) (result *Output, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseOutput(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ParseOutput unmarshals an Output using the given marshalUtil (for easier marshaling/unmarshaling).
func ParseOutput(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*Output) (result *Output, err error) {
	parsedObject, parseErr := marshalUtil.Parse(func(data []byte) (interface{}, int, error) {
		return OutputFromStorageKey(data, optionalTargetObject...)
	})
	if parseErr != nil {
		err = parseErr

		return
	}

	result = parsedObject.(*Output)

	if _, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parsedBytes int, parseErr error) {
		parsedBytes, parseErr = result.UnmarshalObjectStorageValue(data)

		return
	}); err != nil {
		return
	}

	return
}

// OutputFromStorageKey get's called when we restore a Output from the storage.
// In contrast to other database models, it unmarshals some information from the key so we simply store the key before
// it gets handed over to UnmarshalObjectStorageValue (by the ObjectStorage).
func OutputFromStorageKey(keyBytes []byte, optionalTargetObject ...*Output) (result *Output, consumedBytes int, err error) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Output{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to OutputFromStorageKey")
	}

	// parse information
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

// ID returns the identifier of this Output.
func (output *Output) ID() transaction.OutputID {
	return transaction.NewOutputID(output.Address(), output.TransactionID())
}

// Address returns the address that this output belongs to.
func (output *Output) Address() address.Address {
	return output.address
}

// TransactionID returns the id of the Transaction, that created this output.
func (output *Output) TransactionID() transaction.ID {
	return output.transactionID
}

// BranchID returns the id of the ledger state branch, that this output was booked in.
func (output *Output) BranchID() branchmanager.BranchID {
	output.branchIDMutex.RLock()
	defer output.branchIDMutex.RUnlock()

	return output.branchID
}

// SetBranchID is the setter for the property that indicates in which ledger state branch the output is booked.
func (output *Output) SetBranchID(branchID branchmanager.BranchID) (modified bool) {
	output.branchIDMutex.RLock()
	if output.branchID == branchID {
		output.branchIDMutex.RUnlock()

		return
	}

	output.branchIDMutex.RUnlock()
	output.branchIDMutex.Lock()
	defer output.branchIDMutex.Unlock()

	if output.branchID == branchID {
		return
	}

	output.branchID = branchID
	modified = true

	return
}

// Solid returns true if the output has been marked as solid.
func (output *Output) Solid() bool {
	output.solidMutex.RLock()
	defer output.solidMutex.RUnlock()

	return output.solid
}

// SetSolid is the setter of the solid flag. It returns true if the solid flag was modified.
func (output *Output) SetSolid(solid bool) (modified bool) {
	output.solidMutex.RLock()
	if output.solid != solid {
		output.solidMutex.RUnlock()

		output.solidMutex.Lock()
		if output.solid != solid {
			output.solid = solid
			if solid {
				output.solidificationTimeMutex.Lock()
				output.solidificationTime = time.Now()
				output.solidificationTimeMutex.Unlock()
			}

			output.SetModified()

			modified = true
		}
		output.solidMutex.Unlock()

	} else {
		output.solidMutex.RUnlock()
	}

	return
}

// SolidificationTime returns the time when this Output was marked to be solid.
func (output *Output) SolidificationTime() time.Time {
	output.solidificationTimeMutex.RLock()
	defer output.solidificationTimeMutex.RUnlock()

	return output.solidificationTime
}

// RegisterConsumer keeps track of the first transaction, that consumed an Output and consequently keeps track of the
// amount of other transactions spending the same Output.
func (output *Output) RegisterConsumer(consumer transaction.ID) (consumerCount int, firstConsumerID transaction.ID) {
	output.consumerMutex.Lock()
	defer output.consumerMutex.Unlock()

	if consumerCount = output.consumerCount; consumerCount == 0 {
		output.firstConsumer = consumer
	}
	output.consumerCount++

	firstConsumerID = output.firstConsumer

	return
}

// Balances returns the colored balances (color + balance) that this output contains.
func (output *Output) Balances() []*balance.Balance {
	return output.balances
}

// Bytes marshals the object into a sequence of bytes.
func (output *Output) Bytes() []byte {
	return marshalutil.New().
		WriteBytes(output.ObjectStorageKey()).
		WriteBytes(output.ObjectStorageValue()).
		Bytes()
}

// ObjectStorageKey returns the key that is used to store the object in the database.
// It is required to match StorableObject interface.
func (output *Output) ObjectStorageKey() []byte {
	return marshalutil.New(transaction.OutputIDLength).
		WriteBytes(output.address.Bytes()).
		WriteBytes(output.transactionID.Bytes()).
		Bytes()
}

// ObjectStorageValue marshals the balances into a sequence of bytes - the address and transaction id are stored inside the key
// and are ignored here.
func (output *Output) ObjectStorageValue() []byte {
	// determine amount of balances in the output
	balanceCount := len(output.balances)

	// initialize helper
	marshalUtil := marshalutil.New(branchmanager.BranchIDLength + marshalutil.BOOL_SIZE + marshalutil.TIME_SIZE + marshalutil.UINT32_SIZE + balanceCount*balance.Length)
	marshalUtil.WriteBytes(output.branchID.Bytes())
	marshalUtil.WriteBool(output.solid)
	marshalUtil.WriteTime(output.solidificationTime)
	marshalUtil.WriteUint32(uint32(balanceCount))
	for _, balanceToMarshal := range output.balances {
		marshalUtil.WriteBytes(balanceToMarshal.Bytes())
	}

	return marshalUtil.Bytes()
}

// UnmarshalObjectStorageValue restores a Output from a serialized version in the ObjectStorage with parts of the object
// being stored in its key rather than the content of the database to reduce storage requirements.
func (output *Output) UnmarshalObjectStorageValue(data []byte) (consumedBytes int, err error) {
	marshalUtil := marshalutil.New(data)
	if output.branchID, err = branchmanager.ParseBranchID(marshalUtil); err != nil {
		return
	}
	if output.solid, err = marshalUtil.ReadBool(); err != nil {
		return
	}
	if output.solidificationTime, err = marshalUtil.ReadTime(); err != nil {
		return
	}
	var balanceCount uint32
	if balanceCount, err = marshalUtil.ReadUint32(); err != nil {
		return
	}
	output.balances = make([]*balance.Balance, balanceCount)
	for i := uint32(0); i < balanceCount; i++ {
		output.balances[i], err = balance.Parse(marshalUtil)
		if err != nil {
			return
		}
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Update is disabled and panics if it ever gets called - it is required to match StorableObject interface.
func (output *Output) Update(other objectstorage.StorableObject) {
	panic("this object should never be updated")
}

func (output *Output) String() string {
	return stringify.Struct("Output",
		stringify.StructField("address", output.Address()),
		stringify.StructField("transactionId", output.TransactionID()),
		stringify.StructField("branchId", output.BranchID()),
		stringify.StructField("solid", output.Solid()),
		stringify.StructField("solidificationTime", output.SolidificationTime()),
		stringify.StructField("balances", output.Balances()),
	)
}

// define contract (ensure that the struct fulfills the given interface)
var _ objectstorage.StorableObject = &Output{}

// region CachedOutput /////////////////////////////////////////////////////////////////////////////////////////////////

// CachedOutput is a wrapper for the generic CachedObject returned by the objectstorage, that overrides the accessor
// methods, with a type-casted one.
type CachedOutput struct {
	objectstorage.CachedObject
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (cachedOutput *CachedOutput) Unwrap() *Output {
	untypedObject := cachedOutput.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*Output)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (cachedOutput *CachedOutput) Consume(consumer func(output *Output)) (consumed bool) {
	return cachedOutput.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Output))
	})
}

// CachedOutputs represents a collection of CachedOutputs.
type CachedOutputs map[transaction.OutputID]*CachedOutput

// Consume iterates over the CachedObjects, unwraps them and passes a type-casted version to the consumer (if the object
// is not empty - it exists). It automatically releases the object when the consumer finishes. It returns true, if at
// least one object was consumed.
func (cachedOutputs CachedOutputs) Consume(consumer func(output *Output)) (consumed bool) {
	for _, cachedOutput := range cachedOutputs {
		consumed = cachedOutput.Consume(func(output *Output) {
			consumer(output)
		}) || consumed
	}

	return
}

// Release is a utility function, that allows us to release all CachedObjects in the collection.
func (cachedOutputs CachedOutputs) Release(force ...bool) {
	for _, cachedOutput := range cachedOutputs {
		cachedOutput.Release(force...)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
