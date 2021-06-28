package message

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

// DiagnosticBranchesHandler runs the diagnostic over the Tangle.
func DiagnosticBranchesHandler(c echo.Context) (err error) {
	runDiagnosticBranches(c)
	return
}

// DiagnosticLazyBookedBranchesHandler runs the diagnostic over the Tangle.
func DiagnosticLazyBookedBranchesHandler(c echo.Context) (err error) {
	runDiagnosticChildBranches(c, ledgerstate.LazyBookedConflictsBranchID)
	return
}

// DiagnosticInvalidBranchesHandler runs the diagnostic over the Tangle.
func DiagnosticInvalidBranchesHandler(c echo.Context) (err error) {
	runDiagnosticChildBranches(c, ledgerstate.InvalidBranchID)
	return
}

// region DiagnosticBranches code implementation /////////////////////////////////////////////////////////////////////////////////

func runDiagnosticBranches(c echo.Context) {
	// write Header and table description
	c.Response().Header().Set(echo.HeaderContentType, "text/csv")
	c.Response().WriteHeader(http.StatusOK)

	_, err := fmt.Fprintln(c.Response(), strings.Join(DiagnosticBranchesTableDescription, ","))
	if err != nil {
		panic(err)
	}

	messagelayer.Tangle().LedgerState.BranchDAG.ForEachBranch(func(branch ledgerstate.Branch) {
		switch branch.ID() {
		case ledgerstate.MasterBranchID:
			return
		case ledgerstate.InvalidBranchID:
			return
		case ledgerstate.LazyBookedConflictsBranchID:
			return
		default:
			conflictInfo := getDiagnosticConflictsInfo(branch.ID())
			_, err = fmt.Fprintln(c.Response(), conflictInfo.toCSV())
			if err != nil {
				panic(err)
			}
			c.Response().Flush()
		}
	})

	c.Response().Flush()
}

func runDiagnosticChildBranches(c echo.Context, branchID ledgerstate.BranchID) {
	// write Header and table description
	c.Response().Header().Set(echo.HeaderContentType, "text/csv")
	c.Response().WriteHeader(http.StatusOK)

	_, err := fmt.Fprintln(c.Response(), strings.Join(DiagnosticBranchesTableDescription, ","))
	if err != nil {
		panic(err)
	}

	messagelayer.Tangle().LedgerState.BranchDAG.ChildBranches(branchID).Consume(func(childBranch *ledgerstate.ChildBranch) {
		conflictInfo := getDiagnosticConflictsInfo(childBranch.ChildBranchID())
		_, err = fmt.Fprintln(c.Response(), conflictInfo.toCSV())
		if err != nil {
			panic(err)
		}
		c.Response().Flush()
	})

	c.Response().Flush()
}

// DiagnosticBranchesTableDescription holds the description of the diagnostic Branches.
var DiagnosticBranchesTableDescription = []string{
	"ID",
	"ConflictSet",
	"IssuanceTime",
	"SolidTime",
	"OpinionFormedTime",
	"Liked",
	"MonotonicallyLiked",
	"InclusionState",
	"Finalized",
	"LazyBooked",
	"TransactionLiked",
}

// DiagnosticBranchInfo holds the information of a branch.
type DiagnosticBranchInfo struct {
	ID                 string
	ConflictSet        []string
	IssuanceTimestamp  time.Time
	SolidTime          time.Time
	OpinionFormedTime  time.Time
	Liked              bool
	MonotonicallyLiked bool
	InclusionState     string
	Finalized          bool
	SolidityType       string
	TransactionLiked   bool
}

func getDiagnosticConflictsInfo(branchID ledgerstate.BranchID) DiagnosticBranchInfo {
	conflictInfo := DiagnosticBranchInfo{
		ID: branchID.Base58(),
	}

	messagelayer.Tangle().LedgerState.BranchDAG.Branch(branchID).Consume(func(branch ledgerstate.Branch) {
		conflictInfo.Liked = branch.Liked()
		conflictInfo.MonotonicallyLiked = branch.MonotonicallyLiked()
		conflictInfo.InclusionState = messagelayer.Tangle().LedgerState.BranchInclusionState(branchID).String()

		if branch.Type() == ledgerstate.AggregatedBranchType {
			return
		}

		transactionID := ledgerstate.TransactionID(branchID)

		conflictInfo.ConflictSet = messagelayer.Tangle().LedgerState.ConflictSet(transactionID).Base58s()

		messagelayer.Tangle().LedgerState.Transaction(transactionID).Consume(func(transaction *ledgerstate.Transaction) {
			conflictInfo.IssuanceTimestamp = transaction.Essence().Timestamp()
			messagelayer.Tangle().Storage.Attachments(transactionID).Consume(func(attachment *tangle.Attachment) {
				conflictInfo.OpinionFormedTime = messagelayer.ConsensusMechanism().OpinionFormedTime(attachment.MessageID())
			})
		})

		messagelayer.Tangle().LedgerState.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
			conflictInfo.SolidTime = transactionMetadata.SolidificationTime()
			conflictInfo.Finalized = transactionMetadata.Finalized()
			conflictInfo.SolidityType = transactionMetadata.SolidityType().String()
			conflictInfo.TransactionLiked = messagelayer.ConsensusMechanism().TransactionLiked(transactionID)
		})
	})

	return conflictInfo
}

func (d DiagnosticBranchInfo) toCSV() (result string) {
	row := []string{
		d.ID,
		strings.Join(d.ConflictSet, ";"),
		fmt.Sprint(d.IssuanceTimestamp.UnixNano()),
		fmt.Sprint(d.SolidTime.UnixNano()),
		fmt.Sprint(d.OpinionFormedTime.UnixNano()),
		fmt.Sprint(d.Liked),
		fmt.Sprint(d.MonotonicallyLiked),
		d.InclusionState,
		fmt.Sprint(d.Finalized),
		fmt.Sprint(d.SolidityType),
		fmt.Sprint(d.TransactionLiked),
	}

	result = strings.Join(row, ",")

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
