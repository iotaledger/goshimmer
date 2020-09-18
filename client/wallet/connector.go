package wallet

import (
	walletaddr "github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/mana"
)

// Connector represents an interface that defines how the wallet interacts with the network. A wallet can either be used
// locally on a server or it can connect remotely using the web API.
type Connector interface {
	UnspentOutputs(addresses ...walletaddr.Address) (unspentOutputs map[walletaddr.Address]map[transaction.ID]*Output, err error)
	SendTransaction(tx *transaction.Transaction) (err error)
	RequestFaucetFunds(addr walletaddr.Address) (err error)
	GetAllowedPledgeIDs() (pledgeIDMap map[mana.Type][]string, err error)
}
