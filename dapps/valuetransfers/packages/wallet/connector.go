package wallet

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
)

// Connector represents an interface that defines how the wallet interacts with the network. A wallet can either be used
// locally on a server or it can connect remotely using the web API.
type Connector interface {
	UnspentOutputs(addresses ...address.Address) []Output
}
