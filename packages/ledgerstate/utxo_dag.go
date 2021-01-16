package ledgerstate

import (
	"container/list"
	"fmt"
	"strconv"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"github.com/iotaledger/hive.go/typeutils"
	"golang.org/x/xerrors"
)

// region UTXODAG //////////////////////////////////////////////////////////////////////////////////////////////////////

type UTXODAG struct {
	Events *UTXODAGEvents

	transactionStorage          *objectstorage.ObjectStorage
	transactionMetadataStorage  *objectstorage.ObjectStorage
	outputStorage               *objectstorage.ObjectStorage
	outputMetadataStorage       *objectstorage.ObjectStorage
	consumerStorage             *objectstorage.ObjectStorage
	addressOutputMappingStorage *objectstorage.ObjectStorage
	branchDAG                   *BranchDAG
}

func NewUTXODAG(store kvstore.KVStore, branchDAG *BranchDAG) (utxoDAG *UTXODAG) {
	osFactory := objectstorage.NewFactory(store, database.PrefixLedgerState)
	utxoDAG = &UTXODAG{
		Events:                      NewUTXODAGEvents(),
		transactionStorage:          osFactory.New(PrefixTransactionStorage, TransactionFromObjectStorage, transactionStorageOptions...),
		transactionMetadataStorage:  osFactory.New(PrefixTransactionMetadataStorage, TransactionMetadataFromObjectStorage, transactionMetadataStorageOptions...),
		outputStorage:               osFactory.New(PrefixOutputStorage, OutputFromObjectStorage, outputStorageOptions...),
		outputMetadataStorage:       osFactory.New(PrefixOutputMetadataStorage, OutputMetadataFromObjectStorage, outputMetadataStorageOptions...),
		consumerStorage:             osFactory.New(PrefixConsumerStorage, ConsumerFromObjectStorage, consumerStorageOptions...),
		addressOutputMappingStorage: osFactory.New(PrefixAddressOutputMappingStorage, AddressOutputMappingFromObjectStorage, addressOutputMappingStorageOptions...),
		branchDAG:                   branchDAG,
	}
	return
}

func (u *UTXODAG) BookTransaction(transaction *Transaction) (bookTransactionClosure func(), err error) {
	cachedInputs := u.TransactionInputs(transaction)
	defer cachedInputs.Release()
	inputs := cachedInputs.Unwrap()

	if !u.inputsSolid(inputs) {
		err = xerrors.Errorf("not all inputs of transaction are solid: %w", ErrTransactionNotSolid)
		return
	}

	if !u.transactionBalancesValid(inputs, transaction.Essence().Outputs()) {
		err = xerrors.Errorf("sum of consumed and spent balances is not 0: %w", ErrTransactionInvalid)
		return
	}

	if !u.unlockBlocksValid(inputs, transaction) {
		err = xerrors.Errorf("spending of referenced inputs is not authorized: %w", ErrTransactionInvalid)
		return
	}

	if !u.inputsValid(inputs) {
		err = xerrors.Errorf("transaction spends invalid inputs: %w", ErrTransactionInvalid)
		return
	}

	return
}

func (u *UTXODAG) inputsSolid(inputs []Output) (solid bool) {
	for _, input := range inputs {
		if typeutils.IsInterfaceNil(input) {
			// TODO: TRIGGER EVENT FOR EVENTUAL SOLIDIFICATION
			return false
		}
	}

	return true
}

func (u *UTXODAG) transactionBalancesValid(inputs []Output, outputs []Output) (valid bool) {
	// sum up the balances
	consumedBalances := make(map[Color]uint64)
	for _, input := range inputs {
		input.Balances().ForEach(func(color Color, balance uint64) bool {
			consumedBalances[color] += balance

			return true
		})
	}
	for _, output := range outputs {
		output.Balances().ForEach(func(color Color, balance uint64) bool {
			consumedBalances[color] -= balance

			return true
		})
	}

	// check if the balances are all 0
	for _, remainingBalance := range consumedBalances {
		if remainingBalance != 0 {
			return false
		}
	}

	return true
}

func (u *UTXODAG) unlockBlocksValid(inputs []Output, transaction *Transaction) (valid bool) {
	unlockBlocks := transaction.UnlockBlocks()
	for i, input := range inputs {
		unlockValid, unlockErr := input.UnlockValid(transaction, unlockBlocks[i])
		if !unlockValid || unlockErr != nil {
			return false
		}
	}

	return true
}

func (u *UTXODAG) inputsValid(inputs []Output) (valid bool) {
	stack := list.New()
	consumedInputIDs := make(map[OutputID]types.Empty)
	for _, input := range inputs {
		consumedInputIDs[input.ID()] = types.Void
		stack.PushBack(input.ID())
	}

	// TODO: EVENTUALLY RETURN EARLY IF ALL INPUTS ARE UNSPENT (OPTIMIZTAION)

	for stack.Len() > 0 {
		firstElement := stack.Front()
		stack.Remove(firstElement)

		cachedConsumers := u.Consumers(firstElement.Value.(OutputID))
		for _, consumer := range cachedConsumers.Unwrap() {
			if consumer == nil {
				cachedConsumers.Release()
				panic("failed to unwrap Consumer")
			}

			cachedTransaction := u.Transaction(consumer.TransactionID())
			transaction := cachedTransaction.Unwrap()
			if transaction == nil {
				cachedTransaction.Release()
				cachedConsumers.Release()
				panic("failed to unwrap Transaction")
			}

			for _, output := range transaction.Essence().Outputs() {
				if _, exists := consumedInputIDs[output.ID()]; exists {
					cachedTransaction.Release()
					cachedConsumers.Release()
					return false
				}

				stack.PushBack(output.ID())
			}

			cachedTransaction.Release()
		}
		cachedConsumers.Release()
	}

	return true
}

func (u *UTXODAG) Transaction(transactionID TransactionID) (cachedTransaction *CachedTransaction) {
	return &CachedTransaction{CachedObject: u.transactionStorage.Load(transactionID.Bytes())}
}

func (u *UTXODAG) TransactionMetadata(transactionID TransactionID) (cachedTransactionMetadata *CachedTransactionMetadata) {
	return &CachedTransactionMetadata{CachedObject: u.transactionMetadataStorage.Load(transactionID.Bytes())}
}

func (u *UTXODAG) TransactionInputs(transaction *Transaction) (cachedInputs CachedOutputs) {
	cachedInputs = make(CachedOutputs, 0)
	for _, input := range transaction.Essence().Inputs() {
		switch input.Type() {
		case UTXOInputType:
			cachedInputs = append(cachedInputs, u.Output(input.(*UTXOInput).ReferencedOutputID()))
		default:
			panic(fmt.Sprintf("unsupported InputType: %s", input.Type()))
		}
	}

	return
}

func (u *UTXODAG) Output(outputID OutputID) (cachedOutput *CachedOutput) {
	return &CachedOutput{CachedObject: u.outputStorage.Load(outputID.Bytes())}
}

func (u *UTXODAG) OutputsOnAddress(address Address) (cachedOutputsOnAddresses *CachedOutputsOnAddresses) {
	return
}

func (u *UTXODAG) Consumers(outputID OutputID) (cachedConsumers CachedConsumers) {
	cachedConsumers = make(CachedConsumers, 0)
	u.consumerStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedConsumers = append(cachedConsumers, &CachedConsumer{CachedObject: cachedObject})

		return true
	}, outputID.Bytes())

	return
}

func (u *UTXODAG) SetTransactionPreferred(transactionID TransactionID, preferred bool) (modified bool, err error) {
	return
}

func (u *UTXODAG) SetTransactionFinalized(transactionID TransactionID, finalized bool) (modified bool, err error) {
	return
}

func (u *UTXODAG) IsSolid(transaction *Transaction) (solid bool, err error) {
	return
}

func (u *UTXODAG) IsBalancesValid(transaction *Transaction) (valid bool, err error) {
	// check sum of balances == 0

	return
}

func (u *UTXODAG) BranchOfTransaction(transaction *Transaction) (branch BranchID, err error) {
	// check that branches are not conflicting
	// check that transaction does not use an input that was already spent in its past cone

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region UTXODAGEvents ////////////////////////////////////////////////////////////////////////////////////////////////

type UTXODAGEvents struct {
}

func NewUTXODAGEvents() *UTXODAGEvents {
	return &UTXODAGEvents{}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedOutputsOnAddresses /////////////////////////////////////////////////////////////////////////////////////

type CachedOutputsOnAddresses struct {
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AddressOutputMapping /////////////////////////////////////////////////////////////////////////////////////////

func AddressOutputMappingFromObjectStorage(key []byte, data []byte) (result objectstorage.StorableObject, err error) {
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Consumer /////////////////////////////////////////////////////////////////////////////////////////////////////

// Consumer represents the relationship between an Output and its spending Transactions. Since an Output can have a
// potentially unbounded amount of spending Transactions, we store this as a separate k/v pair instead of a marshaled
// list of spending Transactions inside the Output.
type Consumer struct {
	consumedInput OutputID
	transactionID TransactionID

	objectstorage.StorableObjectFlags
}

// NewConsumer creates a Consumer object from the given information.
func NewConsumer(consumedInput OutputID, transactionID TransactionID) *Consumer {
	return &Consumer{
		consumedInput: consumedInput,
		transactionID: transactionID,
	}
}

// ConsumerFromBytes unmarshals a Consumer from a sequence of bytes.
func ConsumerFromBytes(bytes []byte) (consumer *Consumer, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if consumer, err = ConsumerFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Consumer from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ConsumerFromMarshalUtil unmarshals an Consumer using a MarshalUtil (for easier unmarshaling).
func ConsumerFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (consumer *Consumer, err error) {
	consumer = &Consumer{}
	if consumer.consumedInput, err = OutputIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse consumed Input from MarshalUtil: %w", err)
		return
	}
	if consumer.transactionID, err = TransactionIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse TransactionID from MarshalUtil: %w", err)
		return
	}

	return
}

// ConsumerFromObjectStorage is a factory method that creates a new Consumer instance from a storage key of the
// object storage. It is used by the object storage, to create new instances of this entity.
func ConsumerFromObjectStorage(key []byte, _ []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = ConsumerFromBytes(key); err != nil {
		err = xerrors.Errorf("failed to parse Consumer from bytes: %w", err)
		return
	}

	return
}

// ConsumedInput returns the OutputID of the consumed Input.
func (c *Consumer) ConsumedInput() OutputID {
	return c.consumedInput
}

// TransactionID returns the TransactionID of the consuming Transaction.
func (c *Consumer) TransactionID() TransactionID {
	return c.transactionID
}

// Bytes marshals the Consumer into a sequence of bytes.
func (c *Consumer) Bytes() []byte {
	return c.ObjectStorageKey()
}

// String returns a human readable version of the Consumer.
func (c *Consumer) String() (humanReadableConsumer string) {
	return stringify.Struct("Consumer",
		stringify.StructField("consumedInput", c.consumedInput),
		stringify.StructField("transactionID", c.transactionID),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (c *Consumer) Update(other objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (c *Consumer) ObjectStorageKey() []byte {
	return byteutils.ConcatBytes(c.consumedInput.Bytes(), c.transactionID.Bytes())
}

// ObjectStorageValue marshals the Consumer into a sequence of bytes that are used as the value part in the object
// storage.
func (c *Consumer) ObjectStorageValue() []byte {
	panic("implement me")
}

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = &Consumer{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedConsumer ///////////////////////////////////////////////////////////////////////////////////////////////

// CachedConsumer is a wrapper for the generic CachedObject returned by the object storage that overrides the accessor
// methods with a type-casted one.
type CachedConsumer struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedConsumer) Retain() *CachedConsumer {
	return &CachedConsumer{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedConsumer) Unwrap() *Consumer {
	untypedObject := c.Get()
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
func (c *CachedConsumer) Consume(consumer func(consumer *Consumer), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Consumer))
	}, forceRelease...)
}

// String returns a human readable version of the CachedConsumer.
func (c *CachedConsumer) String() string {
	return stringify.Struct("CachedConsumer",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedConsumers //////////////////////////////////////////////////////////////////////////////////////////////

// CachedConsumers represents a collection of CachedConsumer objects.
type CachedConsumers []*CachedConsumer

// Unwrap is the type-casted equivalent of Get. It returns a slice of unwrapped objects with the object being nil if it
// does not exist.
func (c CachedConsumers) Unwrap() (unwrappedOutputs []*Consumer) {
	unwrappedOutputs = make([]*Consumer, len(c))
	for i, cachedConsumer := range c {
		untypedObject := cachedConsumer.Get()
		if untypedObject == nil {
			continue
		}

		typedObject := untypedObject.(*Consumer)
		if typedObject == nil || typedObject.IsDeleted() {
			continue
		}

		unwrappedOutputs[i] = typedObject
	}

	return
}

// Consume iterates over the CachedObjects, unwraps them and passes a type-casted version to the consumer (if the object
// is not empty - it exists). It automatically releases the object when the consumer finishes. It returns true, if at
// least one object was consumed.
func (c CachedConsumers) Consume(consumer func(consumer *Consumer), forceRelease ...bool) (consumed bool) {
	for _, cachedConsumer := range c {
		consumed = cachedConsumer.Consume(consumer, forceRelease...) || consumed
	}

	return
}

// Release is a utility function that allows us to release all CachedObjects in the collection.
func (c CachedConsumers) Release(force ...bool) {
	for _, cachedConsumer := range c {
		cachedConsumer.Release(force...)
	}
}

// String returns a human readable version of the CachedConsumers.
func (c CachedConsumers) String() string {
	structBuilder := stringify.StructBuilder("CachedConsumers")
	for i, cachedConsumer := range c {
		structBuilder.AddField(stringify.StructField(strconv.Itoa(i), cachedConsumer))
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
