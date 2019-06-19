package fcob

import (
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/ternary"
)

// decision rule for setting initial opinion
func decideInitialOpinion(txHash ternary.Trinary, tangle tangleAPI) (opinion opinionState, conflictSet map[ternary.Trinary]bool, err errors.IdentifiableError) {
	// dislikes tx if its past is disliked
	txPast, err := getApproveeLikeStatus(txHash, tangle)
	if err != nil {
		return opinionState{}, conflictSet, err
	}
	if txPast == DISLIKED {
		return opinionState{DISLIKED, VOTED}, conflictSet, nil
	}

	// dislikes tx if it's conflicting
	conflictSet, err = getConflictSet(txHash, tangle)
	if err != nil {
		return opinionState{}, conflictSet, err
	}
	if len(conflictSet) > 0 {
		return opinionState{DISLIKED, UNVOTED}, conflictSet, nil
	}

	// likes tx
	return opinionState{LIKED, UNVOTED}, conflictSet, nil
}

func getApproveeLikeStatus(txHash ternary.Trinary, tangle tangleAPI) (liked bool, err errors.IdentifiableError) {
	// Check branch and trunk finalized like status
	// if at least one is final disliked immidately return dislike FINAL
	txObject, err := tangle.GetTransaction(txHash)
	if err != nil {
		return false, err
	}
	branch := txObject.GetBranchTransactionHash()
	trunk := txObject.GetTrunkTransactionHash()
	approvee := []ternary.Trinary{branch, trunk}
	for _, child := range approvee {
		metadata, err := tangle.GetTransactionMetadata(child)
		if err != nil {
			return false, err
		}
		if metadata != nil && metadata.GetLiked() == false && metadata.GetFinalized() {
			return DISLIKED, nil
		}
	}
	return LIKED, nil
}
