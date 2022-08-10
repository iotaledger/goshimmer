package indexer

import (
	"github.com/cockroachdb/errors"

	"github.com/iotaledger/hive.go/core/cerrors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/objectstorage"

	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/node/database"
)

// region Indexer //////////////////////////////////////////////////////////////////////////////////////////////////////

// Indexer is a component that indexes the Outputs of a ledger for easier lookups.
type Indexer struct {
	// addressOutputMappingStorage is an object storage used to persist AddressOutputMapping objects.
	addressOutputMappingStorage *objectstorage.ObjectStorage[*AddressOutputMapping]

	// ledger contains the indexed Ledger.
	ledger *ledger.Ledger

	// options is a dictionary for configuration parameters of the Indexer.
	options *options
}

// New returns a new Indexer instance with the given options.
func New(ledger *ledger.Ledger, options ...Option) (new *Indexer) {
	new = &Indexer{
		ledger:  ledger,
		options: newOptions(options...),
	}

	new.addressOutputMappingStorage = objectstorage.NewStructStorage[AddressOutputMapping](
		objectstorage.NewStoreWithRealm(new.options.store, database.PrefixIndexer, PrefixAddressOutputMappingStorage),
		new.options.cacheTimeProvider.CacheTime(new.options.addressOutputMappingCacheTime),
		objectstorage.LeakDetectionEnabled(false),
		objectstorage.StoreOnCreation(true),
		objectstorage.PartitionKey(devnetvm.AddressLength, utxo.OutputID{}.Length()),
	)

	ledger.Events.TransactionBooked.Attach(event.NewClosure(new.onTransactionBooked))
	ledger.Events.TransactionAccepted.Attach(event.NewClosure(new.onTransactionAccepted))
	ledger.Events.TransactionRejected.Attach(event.NewClosure(new.onTransactionRejected))

	return new
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

// CachedAddressOutputMappings retrieves all AddressOutputMappings for a particular address
func (i *Indexer) CachedAddressOutputMappings(address devnetvm.Address) (cachedAddressOutputMappings objectstorage.CachedObjects[*AddressOutputMapping]) {
	i.addressOutputMappingStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*AddressOutputMapping]) bool {
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
			return errors.Errorf("failed to prune the object storage (%v): %w", err, cerrors.ErrFatal)
		}
	}

	return
}

// Shutdown shuts down the KVStore used to persist data.
func (i *Indexer) Shutdown() {
	i.addressOutputMappingStorage.Shutdown()
}

// onTransactionBooked adds Transaction outputs to the indexer upon booking.
func (i *Indexer) onTransactionBooked(event *ledger.TransactionBookedEvent) {
	_ = event.Outputs.ForEach(func(output utxo.Output) error {
		i.IndexOutput(output.(devnetvm.Output))
		return nil
	})
}

// onTransactionAccepted removes Transaction inputs from the indexer upon transaction acceptance.
func (i *Indexer) onTransactionAccepted(event *ledger.TransactionAcceptedEvent) {
	i.ledger.Storage.CachedTransaction(event.TransactionID).Consume(func(tx utxo.Transaction) {
		i.removeOutputs(i.ledger.Utils.ResolveInputs(tx.Inputs()))
	})
}

// onTransactionRejected removes Transaction outputs from the indexer upon transaction rejection.
func (i *Indexer) onTransactionRejected(event *ledger.TransactionRejectedEvent) {
	i.ledger.Storage.CachedTransactionMetadata(event.TransactionID).Consume(func(tm *ledger.TransactionMetadata) {
		i.removeOutputs(tm.OutputIDs())
	})
}

// updateOutput applies the passed updateOperation to the provided output.
func (i *Indexer) updateOutput(output devnetvm.Output, updateOperation func(address devnetvm.Address, outputID utxo.OutputID)) {
	switch output.Type() {
	case devnetvm.AliasOutputType:
		castedOutput := output.(*devnetvm.AliasOutput)
		// if it is an origin alias output, we don't have the AliasAddress from the parsed bytes.
		// that happens in ledger output booking, so we calculate the alias address here
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

// removeOutputs removes outputs from the Indexer storage.
func (i *Indexer) removeOutputs(ids utxo.OutputIDs) {
	for it := ids.Iterator(); it.HasNext(); {
		i.ledger.Storage.CachedOutput(it.Next()).Consume(func(o utxo.Output) {
			i.updateOutput(o.(devnetvm.Output), i.RemoveAddressOutputMapping)
		})
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region db prefixes //////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// PrefixAddressOutputMappingStorage defines the storage prefix for the AddressOutputMapping object storage.
	PrefixAddressOutputMappingStorage byte = iota
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
