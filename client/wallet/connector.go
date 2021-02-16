package wallet

import (
	walletaddr "github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// Connector represents an interface that defines how the wallet interacts with the network. A wallet can either be used
// locally on a server or it can connect remotely using the web API.
type Connector interface {
	UnspentOutputs(addresses ...walletaddr.Address) (unspentOutputs map[walletaddr.Address]map[ledgerstate.TransactionID]*Output, err error)
	SendTransaction(tx *ledgerstate.Transaction) (err error)
	RequestFaucetFunds(addr walletaddr.Address) (err error)
}
