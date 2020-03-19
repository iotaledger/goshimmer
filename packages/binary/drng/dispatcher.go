package drng

import (
	"github.com/iotaledger/goshimmer/packages/binary/drng/payload/header"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction"
)

func Dispatch(subtype header.Type, tx *transaction.Transaction) {
	switch subtype {
	case header.CollectiveBeaconType():
		//do stuff
	default:
		//do other stuff
	}
}
