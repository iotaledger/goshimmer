package wallet

import "github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"

type ServerConnector struct {
}

func (s ServerConnector) UnspentOutputs(addresses ...Address) map[Address]map[transaction.ID]Output {
	panic("implement me")
}
