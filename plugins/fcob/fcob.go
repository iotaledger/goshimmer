package fcob

import (
	"github.com/iotaledger/goshimmer/packages/fpc"
	"github.com/iotaledger/goshimmer/packages/ternary"
)

// Updater empty struct used to implement
// the VoterUpdater interface
type Updater struct{}

// RunProtocol defines the signature of function
// implementing the FCoB protocol
type RunProtocol func(txHash ternary.Trinary)

// OpinionUpdater is the interface for updating a tx opinion
type OpinionUpdater interface {
	UpdateOpinion(txHash ternary.Trinary, opinion fpc.Opinion, final bool)
}

// makeRunProtocol returns a runProtocol function as the
// FCoB core logic, that uses the given voter and updater interfaces
func makeRunProtocol(voter fpc.VoteSubmitter, updater OpinionUpdater) RunProtocol {
	// dummy FCoB logic core
	return func(txHash ternary.Trinary) {
		initialOpinion := decisionRule(txHash)
		updater.UpdateOpinion(txHash, initialOpinion, false)
		if initialOpinion == fpc.Dislike {
			voter.SubmitTxsForVoting(fpc.TxOpinion{fpc.ID(txHash), initialOpinion})
		}
	}
}

// decision rule for setting initial opinion
func decisionRule(txHash ternary.Trinary) fpc.Opinion {
	if dummyCheckConflict(txHash) {
		return fpc.Dislike
	}
	return fpc.Like
}

// UpdateOpinion updates the opinion and final flag of a given tx
func (Updater) UpdateOpinion(txHash ternary.Trinary, opinion fpc.Opinion, final bool) {
	// store opinion into cache/db
}

func dummyCheckConflict(txHash ternary.Trinary) bool {
	return true
}
