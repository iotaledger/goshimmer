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

var OutputKeyPartitions = objectstorage.PartitionKey([]int{address.Length, transaction.IdLength}...)

// Output represents the output of a Transaction and contains the balances and the identifiers for this output.
type Output struct {
	address            address.Address
	transactionId      transaction.Id
	branchId           branchmanager.BranchId
	solid              bool
	solidificationTime time.Time
	firstConsumer      transaction.Id
	consumerCount      int
	balances           []*balance.Balance

	branchIdMutex           sync.RWMutex
	solidMutex              sync.RWMutex
	solidificationTimeMutex sync.RWMutex
	consumerMutex           sync.RWMutex

	objectstorage.StorableObjectFlags
	storageKey []byte
}

// NewOutput creates an Output that contains the balances and identifiers of a Transaction.
func NewOutput(address address.Address, transactionId transaction.Id, branchId branchmanager.BranchId, balances []*balance.Balance) *Output {
	return &Output{
		address:            address,
		transactionId:      transactionId,
		branchId:           branchId,
		solid:              false,
		solidificationTime: time.Time{},
		balances:           balances,

		storageKey: marshalutil.New().WriteBytes(address.Bytes()).WriteBytes(transactionId.Bytes()).Bytes(),
	}
}

// OutputFromBytes unmarshals an Output object from a sequence of bytes.
// It either creates a new object or fills the optionally provided object with the parsed information.
func OutputFromBytes(bytes []byte, optionalTargetObject ...*Output) (result *Output, err error, consumedBytes int) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseOutput(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func ParseOutput(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*Output) (result *Output, err error) {
	if parsedObject, parseErr := marshalUtil.Parse(func(data []byte) (interface{}, error, int) {
		return OutputFromStorageKey(data, optionalTargetObject...)
	}); parseErr != nil {
		err = parseErr

		return
	} else {
		result = parsedObject.(*Output)
	}

	if _, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parseErr error, parsedBytes int) {
		parseErr, parsedBytes = result.UnmarshalObjectStorageValue(data)

		return
	}); err != nil {
		return
	}

	return
}

// OutputFromStorageKey get's called when we restore a Output from the storage.
// In contrast to other database models, it unmarshals some information from the key so we simply store the key before
// it gets handed over to UnmarshalObjectStorageValue (by the ObjectStorage).
func OutputFromStorageKey(keyBytes []byte, optionalTargetObject ...*Output) (result *Output, err error, consumedBytes int) {
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
	result.transactionId, err = transaction.ParseId(marshalUtil)
	if err != nil {
		return
	}
	result.storageKey = marshalutil.New(keyBytes[:transaction.OutputIdLength]).Bytes(true)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func (output *Output) Id() transaction.OutputId {
	return transaction.NewOutputId(output.Address(), output.TransactionId())
}

// Address returns the address that this output belongs to.
func (output *Output) Address() address.Address {
	return output.address
}

// TransactionId returns the id of the Transaction, that created this output.
func (output *Output) TransactionId() transaction.Id {
	return output.transactionId
}

// BranchId returns the id of the ledger state branch, that this output was booked in.
func (output *Output) BranchId() branchmanager.BranchId {
	output.branchIdMutex.RLock()
	defer output.branchIdMutex.RUnlock()

	return output.branchId
}

func (output *Output) SetBranchId(branchId branchmanager.BranchId) (modified bool) {
	output.branchIdMutex.RLock()
	if output.branchId == branchId {
		output.branchIdMutex.RUnlock()

		return
	}

	output.branchIdMutex.RUnlock()
	output.branchIdMutex.Lock()
	defer output.branchIdMutex.Unlock()

	if output.branchId == branchId {
		return
	}

	output.branchId = branchId
	modified = true

	return
}

// Solid returns true if the output has been marked as solid.
func (output *Output) Solid() bool {
	output.solidMutex.RLock()
	defer output.solidMutex.RUnlock()

	return output.solid
}

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

func (output *Output) SolidificationTime() time.Time {
	output.solidificationTimeMutex.RLock()
	defer output.solidificationTimeMutex.RUnlock()

	return output.solidificationTime
}

func (output *Output) RegisterConsumer(consumer transaction.Id) (consumerCount int, firstConsumerId transaction.Id) {
	output.consumerMutex.Lock()
	defer output.consumerMutex.Unlock()

	if consumerCount = output.consumerCount; consumerCount == 0 {
		output.firstConsumer = consumer
	}
	output.consumerCount++

	firstConsumerId = output.firstConsumer

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
	return marshalutil.New(transaction.OutputIdLength).
		WriteBytes(output.address.Bytes()).
		WriteBytes(output.transactionId.Bytes()).
		Bytes()
}

// ObjectStorageValue marshals the balances into a sequence of bytes - the address and transaction id are stored inside the key
// and are ignored here.
func (output *Output) ObjectStorageValue() []byte {
	// determine amount of balances in the output
	balanceCount := len(output.balances)

	// initialize helper
	marshalUtil := marshalutil.New(branchmanager.BranchIdLength + marshalutil.BOOL_SIZE + marshalutil.TIME_SIZE + marshalutil.UINT32_SIZE + balanceCount*balance.Length)
	marshalUtil.WriteBytes(output.branchId.Bytes())
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
func (output *Output) UnmarshalObjectStorageValue(data []byte) (err error, consumedBytes int) {
	marshalUtil := marshalutil.New(data)
	if output.branchId, err = branchmanager.ParseBranchId(marshalUtil); err != nil {
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
	} else {
		output.balances = make([]*balance.Balance, balanceCount)
		for i := uint32(0); i < balanceCount; i++ {
			output.balances[i], err = balance.Parse(marshalUtil)
			if err != nil {
				return
			}
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
		stringify.StructField("transactionId", output.TransactionId()),
		stringify.StructField("branchId", output.BranchId()),
		stringify.StructField("solid", output.Solid()),
		stringify.StructField("solidificationTime", output.SolidificationTime()),
		stringify.StructField("balances", output.Balances()),
	)
}

// define contract (ensure that the struct fulfills the given interface)
var _ objectstorage.StorableObject = &Output{}

// region CachedOutput /////////////////////////////////////////////////////////////////////////////////////////////////

type CachedOutput struct {
	objectstorage.CachedObject
}

func (cachedOutput *CachedOutput) Unwrap() *Output {
	if untypedObject := cachedOutput.Get(); untypedObject == nil {
		return nil
	} else {
		if typedObject := untypedObject.(*Output); typedObject == nil || typedObject.IsDeleted() {
			return nil
		} else {
			return typedObject
		}
	}
}

func (cachedOutput *CachedOutput) Consume(consumer func(output *Output)) (consumed bool) {
	return cachedOutput.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Output))
	})
}

type CachedOutputs map[transaction.OutputId]*CachedOutput

func (cachedOutputs CachedOutputs) Consume(consumer func(output *Output)) (consumed bool) {
	for _, cachedOutput := range cachedOutputs {
		consumed = cachedOutput.Consume(func(output *Output) {
			consumer(output)
		}) || consumed
	}

	return
}

func (cachedOutputs CachedOutputs) Release(force ...bool) {
	for _, cachedOutput := range cachedOutputs {
		cachedOutput.Release(force...)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
