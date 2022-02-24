package evilwallet

import (
	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

type OutputStatus int8

const (
	pending OutputStatus = iota
	confirmed
	rejected
)

type Output struct {
	//*wallet.Output
	OutputID ledgerstate.OutputID
	Address  address.Address
	Balance  uint64
	Status   OutputStatus
}

type Outputs []*Output

type OutputManager struct {
}
