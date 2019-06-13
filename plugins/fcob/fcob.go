package fcob

import (
	"github.com/iotaledger/goshimmer/packages/fpc"
	"github.com/iotaledger/goshimmer/packages/ternary"
	"github.com/iotaledger/goshimmer/packages/transaction"
	fpcP "github.com/iotaledger/goshimmer/plugins/fpc"
)

func runProtocol(txHash ternary.Trits) {
	initialOpinion := decisionRule(txHash)
	updateOpinion(txHash, initialOpinion, false)
	if initialOpinion == fpc.Dislike {
		fpcP.INSTANCE.SubmitTxsForVoting(
			fpc.TxOpinion{fpc.ID(txHash.ToString()), initialOpinion})
	}
}

func receiveTransaction(transaction *transaction.Transaction) {
	fpcP.INSTANCE.SubmitTxsForVoting(fpc.TxOpinion{fpc.ID(transaction.Hash.ToString()), fpc.Like})
}

func decisionRule(txHash ternary.Trits) fpc.Opinion {
	if dummyCheckConflict(txHash) {
		return fpc.Dislike
	}
	return fpc.Like
}

func updateOpinion(txHash ternary.Trits, opinion fpc.Opinion, final bool) {
	// store opinion into cache/db
}

func dummyCheckConflict(txHash ternary.Trits) bool {
	return true
}
