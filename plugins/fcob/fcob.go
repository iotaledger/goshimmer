package fcob

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/fpc"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/ternary"
)

const (
	LIKED    = true
	DISLIKED = false
	VOTED    = true
	UNVOTED  = false
)

// RunProtocol defines the signature of the function
// implementing the FCoB protocol
type RunProtocol func(txMetadata ternary.Trinary) error

// opinionState defines the opinion state
type Opinion struct {
	isLiked bool
	isVoted bool
}

// ConflictChecker is the interface for checking if a given tx has conflicts
type ConflictChecker interface {
	GetConflictSet(target ternary.Trinary) (conflictSet map[ternary.Trinary]bool)
}

// makeRunProtocol returns a runProtocol function as the
// FCoB core logic, that uses the given voter and updater interfaces
func makeRunProtocol(plugin *node.Plugin, tangle tangleAPI, voter fpc.Voter) RunProtocol {

	// dummy FCoB logic core
	return func(txHash ternary.Trinary) (err error) {
		// the opinioner decides the initial opinion and the (potential conflict set)
		initialOpinion, conflictSet, err := decideInitialOpinion(txHash, tangle)
		if err != nil {
			return err
		}
		err = setOpinion(txHash, initialOpinion, tangle)
		if err != nil {
			return err
		}
		// don't vote if already voted
		if initialOpinion.isVoted {
			return
		}

		// don't vote if liked
		if initialOpinion.isLiked {
			return
		}

		// submit tx and conflict set for voting
		txsToSubmit := []fpc.TxOpinion{} // list of potential txs to submit for voting
		// we loop over the conflict set
		for tx := range conflictSet {
			txOpinion, err := getOpinion(tx, tangle)
			if err != nil {
				return err
			}
			// include only unvoted txs
			if !txOpinion.isVoted {
				// converting tx into fpc TxLike
				cTx := fpc.TxOpinion{fpc.ID(tx), txOpinion.isLiked}
				txsToSubmit = append(txsToSubmit, cTx)
			}
		}
		voter.SubmitTxsForVoting(txsToSubmit...)

		if plugin != nil {
			plugin.LogInfo(fmt.Sprintf("NewConflict: %v", txsToSubmit))
		}
		return
	}
}

func getOpinion(transactionHash ternary.Trinary, tangle tangleAPI) (opinion Opinion, err errors.IdentifiableError) {
	md, err := tangle.GetTransactionMetadata(transactionHash)
	if err != nil {
		return Opinion{}, err
	}
	return Opinion{md.GetLiked(), md.GetFinalized()}, nil
}

func setOpinion(transactionHash ternary.Trinary, opinion Opinion, tangle tangleAPI) (err errors.IdentifiableError) {
	md, err := tangle.GetTransactionMetadata(transactionHash)
	if err != nil {
		return err
	}
	md.SetLiked(opinion.isLiked)
	md.SetFinalized(opinion.isVoted)
	return nil
}
