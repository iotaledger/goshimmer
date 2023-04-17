package indexer

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/cerrors"
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/objectstorage/generic"
)

// region Indexer //////////////////////////////////////////////////////////////////////////////////////////////////////

// Indexer is a component that indexes the Outputs of a ledgerFunc for easier lookups.
type Indexer struct {
	// addressOutputMappingStorage is an object storage used to persist AddressOutputMapping objects.
	addressOutputMappingStorage *generic.ObjectStorage[*AddressOutputMapping]

	// ledgerFunc contains the indexed MemPool.
	ledgerFunc func() mempool.MemPool

	// options is a dictionary for configuration parameters of the Indexer.
	options *options
}

// New returns a new Indexer instance with the given options.
func New(ledgerFunc func() mempool.MemPool, options ...Option) (i *Indexer) {
	i = &Indexer{
		ledgerFunc: ledgerFunc,
		options:    newOptions(options...),
	}

	i.addressOutputMappingStorage = generic.NewStructStorage[AddressOutputMapping](
		generic.NewStoreWithRealm(i.options.store, database.PrefixIndexer, PrefixAddressOutputMappingStorage),
		i.options.cacheTimeProvider.CacheTime(i.options.addressOutputMappingCacheTime),
		objectstorage.LeakDetectionEnabled(false),
		objectstorage.StoreOnCreation(true),
		objectstorage.PartitionKey(devnetvm.AddressLength, utxo.OutputID{}.Length()),
	)

	return i
}

// IndexOutput stores the AddressOutputMapping dependent on which type of output it is.
func (i *Indexer) IndexOutput(output devnetvm.Output) {
	i.updateOutput(output, i.StoreAddressOutputMapping)
}

// StoreAddressOutputMapping stores the address-output mapping.
func (i *Indexer) StoreAddressOutputMapping(address devnetvm.Address, outputID utxo.OutputID) {
	if result, stored := i.addressOutputMappingStorage.StoreIfAbsent(NewAddressOutputMapping(address, outputID)); stored {
		result.Release()
	}
}

// RemoveAddressOutputMapping removes the address-output mapping.
func (i *Indexer) RemoveAddressOutputMapping(address devnetvm.Address, outputID utxo.OutputID) {
	i.addressOutputMappingStorage.Delete(NewAddressOutputMapping(address, outputID).ObjectStorageKey())
}

// CachedAddressOutputMappings retrieves all AddressOutputMappings for a particular address.
func (i *Indexer) CachedAddressOutputMappings(address devnetvm.Address) (cachedAddressOutputMappings generic.CachedObjects[*AddressOutputMapping]) {
	i.addressOutputMappingStorage.ForEach(func(key []byte, cachedObject *generic.CachedObject[*AddressOutputMapping]) bool {
		cachedAddressOutputMappings = append(cachedAddressOutputMappings, cachedObject)
		return true
	}, objectstorage.WithIteratorPrefix(address.Bytes()))

	return cachedAddressOutputMappings
}

// Prune resets the database and deletes all entities.
func (i *Indexer) Prune() (err error) {
	for _, storagePrune := range []func() error{
		i.addressOutputMappingStorage.Prune,
	} {
		if err = storagePrune(); err != nil {
			return errors.WithMessagef(cerrors.ErrFatal, "failed to prune the object storage: %s", err.Error())
		}
	}

	return
}

// Shutdown shuts down the KVStore used to persist data.
func (i *Indexer) Shutdown() {
	i.addressOutputMappingStorage.Shutdown()
}

// OnOutputCreated adds Transaction outputs to the indexer upon booking.
func (i *Indexer) OnOutputCreated(outputID utxo.OutputID) {
	i.ledgerFunc().Storage().CachedOutput(outputID).Consume(func(o utxo.Output) {
		i.IndexOutput(o.(devnetvm.Output))
	})
}

// OnOutputSpentRejected removes Transaction inputs from the indexer upon transaction acceptance.
func (i *Indexer) OnOutputSpentRejected(outputID utxo.OutputID) {
	i.ledgerFunc().Storage().CachedOutput(outputID).Consume(func(o utxo.Output) {
		i.updateOutput(o.(devnetvm.Output), i.RemoveAddressOutputMapping)
	})
}

// updateOutput applies the passed updateOperation to the provided output.
func (i *Indexer) updateOutput(output devnetvm.Output, updateOperation func(address devnetvm.Address, outputID utxo.OutputID)) {
	switch output.Type() {
	case devnetvm.AliasOutputType:
		castedOutput := output.(*devnetvm.AliasOutput)
		// if it is an origin alias output, we don't have the AliasAddress from the parsed bytes.
		// that happens in ledgerFunc output booking, so we calculate the alias address here
		updateOperation(castedOutput.GetAliasAddress(), output.ID())
		updateOperation(castedOutput.GetStateAddress(), output.ID())
		if !castedOutput.IsSelfGoverned() {
			updateOperation(castedOutput.GetGoverningAddress(), output.ID())
		}
	case devnetvm.ExtendedLockedOutputType:
		castedOutput := output.(*devnetvm.ExtendedLockedOutput)
		if castedOutput.FallbackAddress() != nil {
			updateOperation(castedOutput.FallbackAddress(), output.ID())
		}
		updateOperation(output.Address(), output.ID())
	default:
		updateOperation(output.Address(), output.ID())
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region db prefixes //////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// PrefixAddressOutputMappingStorage defines the storage prefix for the AddressOutputMapping object storage.
	PrefixAddressOutputMappingStorage byte = iota
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
