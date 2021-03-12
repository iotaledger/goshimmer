package tangle

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/datastructure/set"
	"github.com/iotaledger/hive.go/datastructure/walker"
)

// region ApprovalWeightManager ////////////////////////////////////////////////////////////////////////////////////////

type ApprovalWeightManager struct {
	tangle         *Tangle
	lastStatements map[ed25519.PublicKey]*Statement
	supporters     map[ledgerstate.BranchID]set.Set
}

func NewApprovalWeightManager(tangle *Tangle) (approvalWeightManager *ApprovalWeightManager) {
	approvalWeightManager = &ApprovalWeightManager{
		tangle: tangle,
	}

	return
}

func (a *ApprovalWeightManager) ProcessMessage(messageID MessageID) {
	a.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		lastStatement, lastStatementExists := a.lastStatements[message.IssuerPublicKey()]
		if lastStatementExists && !a.isNewStatement(lastStatement, message) {
			return
		}

		newStatement := a.statementFromMessage(message)
		a.lastStatements[message.IssuerPublicKey()] = newStatement

		if lastStatement.BranchID == newStatement.BranchID {
			return
		}

		a.propagateSupportToBranches(newStatement.BranchID, message.IssuerPublicKey())
	})
}

func (a *ApprovalWeightManager) propagateSupportToBranches(branchID ledgerstate.BranchID, issuer ed25519.PublicKey) {
	conflictBranchIDs, err := a.tangle.LedgerState.branchDAG.ResolveConflictBranchIDs(ledgerstate.NewBranchIDs(branchID))
	if err != nil {
		panic(err)
	}

	supportWalker := walker.New()
	for conflictBranchID := range conflictBranchIDs {
		supportWalker.Push(conflictBranchID)
	}

	for supportWalker.HasNext() {
		a.addSupportToBranch(supportWalker.Next().(ledgerstate.BranchID), issuer, supportWalker)
	}
}

func (a *ApprovalWeightManager) addSupportToBranch(branchID ledgerstate.BranchID, issuer ed25519.PublicKey, walk *walker.Walker) {
	if !a.supporters[branchID].Add(issuer) {
		return
	}

	a.tangle.LedgerState.branchDAG.ForEachConflictingBranchID(branchID, func(conflictingBranchID ledgerstate.BranchID) {
		revokeWalker := walker.New()
		revokeWalker.Push(conflictingBranchID)

		for revokeWalker.HasNext() {
			a.revokeSupportFromBranch(revokeWalker.Next().(ledgerstate.BranchID), issuer, revokeWalker)
		}
	})
}

func (a *ApprovalWeightManager) revokeSupportFromBranch(branchID ledgerstate.BranchID, issuer ed25519.PublicKey, walker *walker.Walker) {
	if !a.supporters[branchID].Delete(issuer) {
		return
	}

	a.tangle.LedgerState.branchDAG.ChildBranches(branchID).Consume(func(childBranch *ledgerstate.ChildBranch) {
		if childBranch.ChildBranchType() != ledgerstate.ConflictBranchType {
			return
		}

		walker.Push(childBranch.ChildBranchID())
	})
}

func (a *ApprovalWeightManager) isNewStatement(lastStatement *Statement, message *Message) bool {
	return lastStatement.SequenceNumber < message.SequenceNumber()
}

func (a *ApprovalWeightManager) statementFromMessage(message *Message) (statement *Statement) {
	if !a.tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
		statement = &Statement{
			SequenceNumber: message.SequenceNumber(),
			BranchID:       messageMetadata.BranchID(),
		}

	}) {
		panic(fmt.Errorf("failed to load MessageMetadata of Message with %s", message.ID()))
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Statement ////////////////////////////////////////////////////////////////////////////////////////////////////

type Statement struct {
	SequenceNumber uint64
	BranchID       ledgerstate.BranchID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
