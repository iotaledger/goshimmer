package indexer

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/marshalutil"

	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
)

// region AddressOutputMapping /////////////////////////////////////////////////////////////////////////////////////////

// AddressOutputMapping is a mapping from an Address to an OutputID than enables lookups of stored Outputs.
type AddressOutputMapping struct {
	address  devnetvm.Address
	outputID utxo.OutputID

	objectstorage.StorableObjectFlags
}

// NewAddressOutputMapping creates a new AddressOutputMapping.
func NewAddressOutputMapping(address devnetvm.Address, outputID utxo.OutputID) *AddressOutputMapping {
	return &AddressOutputMapping{
		address:  address,
		outputID: outputID,
	}
}

// Address returns the Address of the AddressOutputMapping.
func (a *AddressOutputMapping) Address() devnetvm.Address {
	return a.address
}

// OutputID returns the OutputID of the AddressOutputMapping.
func (a *AddressOutputMapping) OutputID() utxo.OutputID {
	return a.outputID
}

// FromObjectStorage un-serializes an AddressOutputMapping from an object storage.
func (a *AddressOutputMapping) FromObjectStorage(key, _ []byte) (addressOutputMapping objectstorage.StorableObject, err error) {
	result := new(AddressOutputMapping)
	if err = result.FromMarshalUtil(marshalutil.New(key)); err != nil {
		return nil, err
	}

	return result, nil
}

// FromMarshalUtil un-serializes an AddressOutputMapping using a MarshalUtil.
func (a *AddressOutputMapping) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	if a.address, err = devnetvm.AddressFromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("failed to parse consumed Address from MarshalUtil: %w", err)
	}
	if err = a.outputID.FromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("failed to parse OutputID from MarshalUtil: %w", err)
	}

	return
}

// ObjectStorageKey serializes the part of the object that is stored in the key part of the object storage.
func (a *AddressOutputMapping) ObjectStorageKey() (key []byte) {
	return byteutils.ConcatBytes(a.address.Bytes(), a.outputID.Bytes())
}

// ObjectStorageValue serializes the part of the object that is stored in the value part of the object storage.
func (a *AddressOutputMapping) ObjectStorageValue() (value []byte) {
	return nil
}

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = new(AddressOutputMapping)

// addressOutputMappingPartitionKeys defines the partition of the storage key of the AddressOutputMapping model.
var addressOutputMappingPartitionKeys = objectstorage.PartitionKey([]int{devnetvm.AddressLength, utxo.TransactionIDLength}...)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
