package wallet

import (
	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// Connector represents an interface that defines how the wallet interacts with the network. A wallet can either be used
// locally on a server or it can connect remotely using the web API.
type Connector interface {
	UnspentOutputs(addresses ...address.Address) (unspentOutputs map[address.Address]map[ledgerstate.OutputID]*Output, err error)
	SendTransaction(transaction *ledgerstate.Transaction) (err error)
	RequestFaucetFunds(address address.Address) (err error)
}
