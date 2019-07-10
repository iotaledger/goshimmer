package fcob

import (
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/iota.go/trinary"
)

// decision rule for setting initial opinion
func decideInitialOpinion(txHash trinary.Trytes, tangle tangleAPI) (opinion Opinion, conflictSet map[trinary.Trytes]bool, err errors.IdentifiableError) {
	// dislikes tx if its past is disliked
	txPastLiked, err := getApproveeLikeStatus(txHash, tangle)
	if err != nil {
		return Opinion{}, conflictSet, err
	}
	if !txPastLiked {
		return Opinion{DISLIKED, VOTED}, conflictSet, nil
	}

	// dislikes tx if it's conflicting
	conflictSet, err = getConflictSet(txHash, tangle)
	if err != nil {
		return Opinion{}, conflictSet, err
	}
	if len(conflictSet) > 0 {
		return Opinion{DISLIKED, UNVOTED}, conflictSet, nil
	}

	// likes tx
	return Opinion{LIKED, UNVOTED}, conflictSet, nil
}

func getApproveeLikeStatus(txHash trinary.Trytes, tangle tangleAPI) (liked bool, err errors.IdentifiableError) {
	// Check branch and trunk voted like status
	// if at least one is voted disliked, returns disliked voted
	txObject, err := tangle.GetTransaction(txHash)
	if err != nil {
		return false, err
	}
	branch := txObject.GetBranchTransactionHash()
	trunk := txObject.GetTrunkTransactionHash()
	approvee := []trinary.Trytes{branch, trunk}
	for _, child := range approvee {
		metadata, err := tangle.GetTransactionMetadata(child)
		if err != nil {
			return false, err
		}
		if metadata != nil && !metadata.GetLiked() && metadata.GetFinalized() {
			return DISLIKED, nil
		}
	}
	return LIKED, nil
}
