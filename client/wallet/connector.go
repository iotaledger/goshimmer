package wallet

import (
	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/mana"
)

// Connector represents an interface that defines how the wallet interacts with the network. A wallet can either be used
// locally on a server or it can connect remotely using the web API.
type Connector interface {
	UnspentOutputs(addresses ...address.Address) (unspentOutputs OutputsByAddressAndOutputID, err error)
	SendTransaction(transaction *ledgerstate.Transaction) (err error)
	RequestFaucetFunds(address address.Address, powTarget int) (err error)
	GetAllowedPledgeIDs() (pledgeIDMap map[mana.Type][]string, err error)
	GetTransactionGoF(txID ledgerstate.TransactionID) (gradeOfFinality gof.GradeOfFinality, err error)
	GetUnspentAliasOutput(address *ledgerstate.AliasAddress) (output *ledgerstate.AliasOutput, err error)
}
