package fcob

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/fpc"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/iota.go/trinary"
)

const (
	LIKED    = true
	DISLIKED = false
	VOTED    = true
	UNVOTED  = false
)

// RunProtocol defines the signature of the function
// implementing the FCoB protocol
type RunProtocol func(txMetadata trinary.Trytes)

// Opinion defines the opinion state
type Opinion struct {
	isLiked bool
	isVoted bool
}

// ConflictChecker is the interface for checking if a given tx has conflicts
type ConflictChecker interface {
	GetConflictSet(target trinary.Trytes) (conflictSet map[trinary.Trytes]bool)
}

func configureFCOB(plugin *node.Plugin, tangle tangleAPI, voter fpc.Voter) *events.Closure {
	runFCOB := makeRunProtocol(plugin, tangle, voter)
	return events.NewClosure(func(transaction *value_transaction.ValueTransaction) {
		// start as a goroutine so that immediately returns
		go runFCOB(transaction.GetHash())
	})
}

// makeRunProtocol returns a runProtocol function as the
// FCoB core logic, that uses the given voter and updater interfaces
func makeRunProtocol(plugin *node.Plugin, tangle tangleAPI, voter fpc.Voter) RunProtocol {
	// FCoB logic core
	return func(txHash trinary.Trytes) {
		// the opinioner decides the initial opinion and the (potential conflict set)
		initialOpinion, conflictSet, err := decideInitialOpinion(txHash, tangle)
		if err != nil {
			plugin.LogFailure(fmt.Sprint(err))
			return
		}
		err = setOpinion(txHash, initialOpinion, tangle)
		if err != nil {
			plugin.LogFailure(fmt.Sprint(err))
			return
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
				plugin.LogFailure(fmt.Sprint(err))
				return
			}
			// include only unvoted txs
			if !txOpinion.isVoted {
				// converting tx into fpc TxOpinion
				cTx := fpc.TxOpinion{
					TxHash:  fpc.ID(tx),
					Opinion: txOpinion.isLiked,
				}
				txsToSubmit = append(txsToSubmit, cTx)
			}
		}
		voter.SubmitTxsForVoting(txsToSubmit...)

		if plugin != nil {
			plugin.LogInfo(fmt.Sprintf("NewConflict: %v", txsToSubmit))
		}
	}

}

func configureUpdateTxsVoted(plugin *node.Plugin, tangle tangleAPI) *events.Closure {
	return events.NewClosure(func(txs []fpc.TxOpinion) {
		plugin.LogInfo(fmt.Sprintf("Voting Done for txs: %v", txs))
		for _, tx := range txs {
			err := setOpinion(trinary.Trytes(tx.TxHash), Opinion{tx.Opinion, VOTED}, tangle)
			if err != nil {
				plugin.LogFailure(fmt.Sprint(err))
			}
		}
	})
}

func getOpinion(transactionHash trinary.Trytes, tangle tangleAPI) (opinion Opinion, err errors.IdentifiableError) {
	md, err := tangle.GetTransactionMetadata(transactionHash)
	if err != nil {
		return Opinion{}, err
	}
	return Opinion{md.GetLiked(), md.GetFinalized()}, nil
}

func setOpinion(transactionHash trinary.Trytes, opinion Opinion, tangle tangleAPI) (err errors.IdentifiableError) {
	md, err := tangle.GetTransactionMetadata(transactionHash)
	if err != nil {
		return err
	}
	md.SetLiked(opinion.isLiked)
	md.SetFinalized(opinion.isVoted)
	return nil
}
