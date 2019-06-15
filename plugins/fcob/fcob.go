package fcob

import (
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/fpc"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/ternary"
)

var beingVoted *VotingnMap

// Fcob struct used to implement
// the VoterUpdater interface
type Fcob struct {
	plugin *node.Plugin
}

// RunProtocol defines the signature of function
// implementing the FCoB protocol
type RunProtocol func(txHash ternary.Trinary)

// Opinioner is the interface for updating an opinion
type Opinioner interface {
	GetOpinion(transactionHash ternary.Trinary) (opinion Opinion, err errors.IdentifiableError)
	SetOpinion(transactionHash ternary.Trinary, opinion Opinion) (err errors.IdentifiableError)
	Decide(txHash ternary.Trinary) (opinion Opinion, conflictSet map[ternary.Trinary]bool)
}

// ConflictChecker is the interface for checking if a given tx has conflicts
type ConflictChecker interface {
	Check(tx ternary.Trinary) (fpc.Opinion, []ternary.Trinary)
}

type dummyMetadata struct {
	like  bool
	voted bool
}
type dummyTransaction struct {
	txHash         ternary.Trinary
	branch         ternary.Trinary
	trunk          ternary.Trinary
	address        ternary.Trinary
	metaData       *dummyMetadata
	solidTimestamp uint64
}

type dummyConflict struct{}

var dummyTangle map[ternary.Trinary]dummyTransaction

// makeRunProtocol returns a runProtocol function as the
// FCoB core logic, that uses the given voter and updater interfaces
func makeRunProtocol(voter fpc.Voter, opinioner Opinioner) RunProtocol {
	// init being voted map
	beingVoted = NewVotingnMap()

	// dummy FCoB logic core
	return func(txHash ternary.Trinary) {
		initialOpinion, conflictSet := opinioner.Decide(txHash)
		err := opinioner.SetOpinion(txHash, initialOpinion)
		if err != nil {
			//TODO: handle error
		}
		if !initialOpinion.liked() && !initialOpinion.voted() {

			// converting txHash into fpc TxOpinion

			txsToVote := []fpc.TxOpinion{}
			for tx := range conflictSet {
				//TODO: add && is not being already in the process of voting
				txOpinion, err := opinioner.GetOpinion(tx)
				if err != nil {
					//TODO: handle error
				}
				if !txOpinion.voted() && !beingVoted.Load(tx) {
					cTx := fpc.TxOpinion{fpc.ID(tx), txOpinion.like}
					txsToVote = append(txsToVote, cTx)
					beingVoted.Store(tx)
				}
			}
			//TODO: add check that at least one conflicting tx is voted [maybe include like status]
			// if that is the case, update the voted status for txHash to true and skip voting
			// else do the voting
			voter.SubmitTxsForVoting(txsToVote...)
		}
	}
}

// TODO: change Check into getConflictSet
func (dummyConflict) GetConflictSet(target ternary.Trinary) (conflictSet map[ternary.Trinary]bool) {
	targetAddress := dummyTangle[target].address
	conflictSet = make(map[ternary.Trinary]bool)
	conflict := false
	// In real implementation we don't need to iterate the all tangle
	// since we can just use the ledger state.
	for txHash, txObject := range dummyTangle {
		if targetAddress == txObject.address {
			if target != txHash { // filter out the same target tx
				conflictSet[txHash] = true
				conflict = true
			}
		}
	}
	if conflict {
		conflictSet[target] = true
	}
	return conflictSet
}

func dummyCheckConflict(txHash ternary.Trinary) bool {
	return true
}
