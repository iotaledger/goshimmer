package wallet

import "github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"

// ServerConnector is a connector that uses local instance of a node instead of the remote api to implement the required
// functions for the wallet.
type ServerConnector struct {
}

// SendTransaction sends a transaction into the network
func (s ServerConnector) SendTransaction(tx *transaction.Transaction) {
	panic("implement me")
}

// UnspentOutputs returns the unspent outputs on the given addresses.
func (s ServerConnector) UnspentOutputs(addresses ...Address) map[Address]map[transaction.ID]*Output {
	panic("implement me")
}
