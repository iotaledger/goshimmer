package wallet

import (
	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
)

// Connector represents an interface that defines how the wallet interacts with the network. A wallet can either be used
// locally on a server or it can connect remotely using the web API.
type Connector interface {
	UnspentOutputs(addresses ...address.Address) (unspentOutputs OutputsByAddressAndOutputID, err error)
	SendTransaction(transaction *devnetvm.Transaction) (err error)
	RequestFaucetFunds(address address.Address, powTarget int) (err error)
	GetTransactionConfirmationState(txID utxo.TransactionID) (confirmationState confirmation.State, err error)
	GetUnspentAliasOutput(address *devnetvm.AliasAddress) (output *devnetvm.AliasOutput, err error)
}
