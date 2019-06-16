package fcob

import (
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/fpc"
	"github.com/iotaledger/goshimmer/packages/ternary"
)

// beingVoted is the voting map containing all the txs
// currently being voted
var beingVoted *VotingnMap

// RunProtocol defines the signature of the function
// implementing the FCoB protocol
type RunProtocol func(txMetadata ternary.Trinary) error

// Opinioner is the interface defining the behaviour for
// - getting the opinion for a given tx
// - setting the opinion of a given
// - decide the opinion of a given tx
type Opinioner interface {
	GetOpinion(transactionHash ternary.Trinary) (opinion Opinion, err errors.IdentifiableError)
	SetOpinion(transactionHash ternary.Trinary, opinion Opinion) (err errors.IdentifiableError)
	Decide(txHash ternary.Trinary) (opinion Opinion, conflictSet map[ternary.Trinary]bool, err errors.IdentifiableError)
}

// ConflictChecker is the interface for checking if a given tx has conflicts
type ConflictChecker interface {
	GetConflictSet(target ternary.Trinary) (conflictSet map[ternary.Trinary]bool)
}

// makeRunProtocol returns a runProtocol function as the
// FCoB core logic, that uses the given voter and updater interfaces
func makeRunProtocol(voter fpc.Voter, opinioner Opinioner) RunProtocol {
	// init being voted map
	beingVoted = NewVotingMap()

	// dummy FCoB logic core
	return func(txHash ternary.Trinary) (err error) {
		// the opinioner decides the initial opinion and the (potential conflict set)
		initialOpinion, conflictSet, err := opinioner.Decide(txHash)
		if err != nil {
			return err
		}
		err = opinioner.SetOpinion(txHash, initialOpinion)
		if err != nil {
			return err
		}
		// the opinioner disliked this tx
		if !initialOpinion.liked() && !initialOpinion.voted() {

			txsToSubmit := []fpc.TxOpinion{} // list of potential txs to submit for voting
			// we loop over the conflict set
			for tx := range conflictSet {
				txOpinion, err := opinioner.GetOpinion(tx)
				if err != nil {
					return err
				}
				// check not already voted AND not currently being voted
				if !txOpinion.voted() && !beingVoted.Load(tx) {
					// converting tx into fpc TxOpinion
					cTx := fpc.TxOpinion{fpc.ID(tx), txOpinion.like}
					txsToSubmit = append(txsToSubmit, cTx)
					beingVoted.Store(tx) // adding it to the voting map
				}
			}
			//TODO: add check that at least one conflicting tx is voted [maybe include like status]
			// if that is the case, update the voted status for txHash to true and skip voting
			// else do the voting
			voter.SubmitTxsForVoting(txsToSubmit...)
		}
		return
	}
}
