package wallet

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

// Connector represents an interface that defines how the wallet interacts with the network. A wallet can either be used
// locally on a server or it can connect remotely using the web API.
type Connector interface {
	UnspentOutputs(addresses ...Address) map[Address]map[transaction.ID]Output
	SendTransaction(tx *transaction.Transaction)
}
