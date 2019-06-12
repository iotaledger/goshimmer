package fcob

import (
	"github.com/iotaledger/goshimmer/packages/fpc"
	"github.com/iotaledger/goshimmer/packages/transaction"
)

type Fcob struct {
	fpc fpc.FPC
}

func (fcob Fcob) ReceiveTransaction(f Fpc, transaction *transaction.Transaction) {
	fcob.fpc.VoteOnTxs(fpc.TxOpinion{true, transaction.Hash})
}
