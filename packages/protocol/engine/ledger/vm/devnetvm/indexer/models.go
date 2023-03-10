package indexer

import (
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/hive.go/objectstorage/generic/model"
)

// region AddressOutputMapping /////////////////////////////////////////////////////////////////////////////////////////

// AddressOutputMapping is a mapping from an Address to an OutputID than enables lookups of stored Outputs.
type AddressOutputMapping struct {
	model.StorableReference[AddressOutputMapping, *AddressOutputMapping, devnetvm.Address, utxo.OutputID] `serix:"0"`
}

// NewAddressOutputMapping creates a new AddressOutputMapping.
func NewAddressOutputMapping(address devnetvm.Address, outputID utxo.OutputID) *AddressOutputMapping {
	return model.NewStorableReference[AddressOutputMapping](address, outputID)
}

// Address returns the Address of the AddressOutputMapping.
func (a *AddressOutputMapping) Address() devnetvm.Address {
	return a.SourceID()
}

// OutputID returns the OutputID of the AddressOutputMapping.
func (a *AddressOutputMapping) OutputID() utxo.OutputID {
	return a.TargetID()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
