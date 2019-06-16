package fcob

import (
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/fpc"
	"github.com/iotaledger/goshimmer/packages/ternary"
	"github.com/iotaledger/goshimmer/plugins/tangle"
)

// tangleHook is an empty struct used to implement
// the interface Opinioner by using the real Tangle
type tangleHook struct{}

func (tangleHook) GetOpinion(transactionHash ternary.Trinary) (opinion Opinion, err errors.IdentifiableError) {
	md, err := tangle.GetTransactionMetadata(transactionHash)
	if err != nil {
		return Opinion{}, err
	}
	return Opinion{boolToOpinion(md.GetLiked()), md.GetFinalized()}, nil
}

func (tangleHook) SetOpinion(transactionHash ternary.Trinary, opinion Opinion) (err errors.IdentifiableError) {
	md, err := tangle.GetTransactionMetadata(transactionHash)
	if err != nil {
		return err
	}
	md.SetLiked(opinionToBool(opinion))
	md.SetFinalized(opinion.final)
	return nil
}

// decision rule for setting initial opinion
func (tangleHook) Decide(txHash ternary.Trinary) (opinion Opinion, conflictSet map[ternary.Trinary]bool, err errors.IdentifiableError) {
	// Check branch and trunk finalized like status
	// if at least one is final disliked immidately return dislike FINAL
	txObject, err := tangle.GetTransaction(txHash)
	if err != nil {
		return Opinion{}, conflictSet, err
	}
	branch := txObject.GetBranchTransactionHash()
	trunk := txObject.GetBranchTransactionHash()
	approvee := []ternary.Trinary{branch, trunk}
	for _, child := range approvee {
		metadata, err := tangle.GetTransactionMetadata(child)
		if err != nil {
			return Opinion{}, conflictSet, err
		}
		if metadata != nil && metadata.GetLiked() == false && metadata.GetFinalized() {
			return Opinion{fpc.Dislike, true}, conflictSet, nil
		}
	}
	// TODO: change dummyConflict with the real conflict checker
	conflictSet, err = dummyConflict{}.GetConflictSet(txHash)
	if err != nil {
		return Opinion{}, conflictSet, err
	}
	if len(conflictSet) > 0 {
		return Opinion{fpc.Dislike, false}, conflictSet, nil
	}
	return Opinion{fpc.Like, false}, conflictSet, nil
}
