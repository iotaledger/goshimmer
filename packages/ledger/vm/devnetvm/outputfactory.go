package devnetvm

import (
	"github.com/iotaledger/hive.go/marshalutil"

	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
)

func OutputFactory(marshalUtil *marshalutil.MarshalUtil) (output utxo.Output, err error) {
	return OutputFromBytes(marshalUtil.Bytes())
}
