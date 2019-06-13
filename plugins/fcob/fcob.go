package fcob

import (
	"github.com/iotaledger/goshimmer/packages/fpc"
	"github.com/iotaledger/goshimmer/packages/ternary"
	fpcP "github.com/iotaledger/goshimmer/plugins/fpc"
)

// dummy FCoB logic core
func runProtocol(txHash ternary.Trits) {
	initialOpinion := decisionRule(txHash)
	updateOpinion(txHash, initialOpinion, false)
	if initialOpinion == fpc.Dislike {
		fpcP.INSTANCE.SubmitTxsForVoting(
			fpc.TxOpinion{fpc.ID(txHash.ToString()), initialOpinion})
	}
}

// decision rule for setting initial opinion
func decisionRule(txHash ternary.Trits) fpc.Opinion {
	if dummyCheckConflict(txHash) {
		return fpc.Dislike
	}
	return fpc.Like
}

// udpates the opinion and final flag of a given tx
func updateOpinion(txHash ternary.Trits, opinion fpc.Opinion, final bool) {
	// store opinion into cache/db
}

// dummy conflict checker
func dummyCheckConflict(txHash ternary.Trits) bool {
	return true
}
