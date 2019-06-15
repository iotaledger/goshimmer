package fcob

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/fpc"
	"github.com/iotaledger/goshimmer/packages/ternary"
	"github.com/iotaledger/goshimmer/plugins/tangle"
)

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
func (tangleHook) Decide(txHash ternary.Trinary) (opinion Opinion, conflictSet map[ternary.Trinary]bool) {
	// Check branch and trunk finalized like status
	// if at least one is final disliked immidately return dislike FINAL
	// txObject, err := tangle.GetTransaction(txHash)
	// if err != nil {
	// 	//TODO: handle error
	// 	PLUGIN.LogFailure("tangle.GetTransaction(txHash)")
	// }
	// branch := txObject.GetBranchTransactionHash()
	// trunk := txObject.GetBranchTransactionHash()
	// approvee := []ternary.Trinary{branch, trunk}
	// for _, child := range approvee {
	// 	metadata, err := tangle.GetTransactionMetadata(child)
	// 	if err != nil {
	// 		//TODO: handle error
	// 		PLUGIN.LogFailure("tangle.GetTransactionMetadata(child)")
	// 	}
	// 	PLUGIN.LogFailure(fmt.Sprintf("metadata: %v, branch: %v, trunk: %v", metadata, branch, trunk))
	// 	if metadata.GetLiked() == false && metadata.GetFinalized() {
	// 		return Opinion{fpc.Dislike, true}, conflictSet
	// 	}
	// }
	PLUGIN.LogInfo(fmt.Sprintf("(decide) TxHash: %v", txHash))
	conflictSet = dummyConflict{}.GetConflictSet(txHash)
	if len(conflictSet) > 0 {
		return Opinion{fpc.Dislike, false}, conflictSet
	}
	return Opinion{fpc.Like, false}, conflictSet
}
