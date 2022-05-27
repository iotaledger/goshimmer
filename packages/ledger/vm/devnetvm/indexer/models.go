package indexer

import (
	"github.com/iotaledger/hive.go/generics/model"

	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
)

// region AddressOutputMapping /////////////////////////////////////////////////////////////////////////////////////////

// AddressOutputMapping is a mapping from an Address to an OutputID than enables lookups of stored Outputs.
type AddressOutputMapping struct {
	model.StorableReference[devnetvm.Address, utxo.OutputID]
}

// NewAddressOutputMapping creates a new AddressOutputMapping.
func NewAddressOutputMapping(address devnetvm.Address, outputID utxo.OutputID) *AddressOutputMapping {
	return &AddressOutputMapping{model.NewStorableReference(address, outputID)}
}

// Address returns the Address of the AddressOutputMapping.
func (a *AddressOutputMapping) Address() devnetvm.Address {
	return a.SourceID
}

// OutputID returns the OutputID of the AddressOutputMapping.
func (a *AddressOutputMapping) OutputID() utxo.OutputID {
	return a.TargetID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
