package indexer

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/objectstorage"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
)

// region Indexer //////////////////////////////////////////////////////////////////////////////////////////////////////

// Indexer is a component that indexes the Outputs of a ledger for easier lookups.
type Indexer struct {
	// addressOutputMappingStorage is an object storage used to persist AddressOutputMapping objects.
	addressOutputMappingStorage *objectstorage.ObjectStorage[*AddressOutputMapping]

	// options is a dictionary for configuration parameters of the Indexer.
	options *options
}

// New returns a new Indexer instance with the given options.
func New(options ...Option) (new *Indexer) {
	new = &Indexer{
		options: newOptions(options...),
	}
	new.addressOutputMappingStorage = objectstorage.New[*AddressOutputMapping](
		new.options.store.WithRealm([]byte{database.PrefixIndexer, PrefixAddressOutputMappingStorage}),
		new.options.cacheTimeProvider.CacheTime(new.options.addressOutputMappingCacheTime),
		objectstorage.LeakDetectionEnabled(false),
		objectstorage.StoreOnCreation(true),
		addressOutputMappingPartitionKeys,
	)

	return new
}

// IndexOutput stores the AddressOutputMapping dependent on which type of output it is.
func (i *Indexer) IndexOutput(output devnetvm.Output) {
	switch output.Type() {
	case devnetvm.AliasOutputType:
		castedOutput := output.(*devnetvm.AliasOutput)
		// if it is an origin alias output, we don't have the AliasAddress from the parsed bytes.
		// that happens in ledger output booking, so we calculate the alias address here
		i.StoreAddressOutputMapping(castedOutput.GetAliasAddress(), output.ID())
		i.StoreAddressOutputMapping(castedOutput.GetStateAddress(), output.ID())
		if !castedOutput.IsSelfGoverned() {
			i.StoreAddressOutputMapping(castedOutput.GetGoverningAddress(), output.ID())
		}
	case devnetvm.ExtendedLockedOutputType:
		castedOutput := output.(*devnetvm.ExtendedLockedOutput)
		if castedOutput.FallbackAddress() != nil {
			i.StoreAddressOutputMapping(castedOutput.FallbackAddress(), output.ID())
		}
		i.StoreAddressOutputMapping(output.Address(), output.ID())
	default:
		i.StoreAddressOutputMapping(output.Address(), output.ID())
	}
}

// StoreAddressOutputMapping stores the address-output mapping.
func (i *Indexer) StoreAddressOutputMapping(address devnetvm.Address, outputID utxo.OutputID) {
	if result, stored := i.addressOutputMappingStorage.StoreIfAbsent(NewAddressOutputMapping(address, outputID)); stored {
		result.Release()
	}
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region db prefixes //////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// PrefixAddressOutputMappingStorage defines the storage prefix for the AddressOutputMapping object storage.
	PrefixAddressOutputMappingStorage byte = iota
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
