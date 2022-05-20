package message

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/conflictdag"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
)

// DiagnosticBranchesHandler runs the diagnostic over the Tangle.
func DiagnosticBranchesHandler(c echo.Context) (err error) {
	runDiagnosticBranches(c)
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

	deps.Tangle.Ledger.ConflictDAG.Utils.ForEachBranch(func(branch *conflictdag.Branch[utxo.TransactionID, utxo.OutputID]) {
		switch branch.ID() {
		case utxo.EmptyTransactionID:
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

// DiagnosticBranchesTableDescription holds the description of the diagnostic Branches.
var DiagnosticBranchesTableDescription = []string{
	"ID",
	"ConflictSet",
	"IssuanceTime",
	"BookingTime",
	"LazyBooked",
	"GradeOfFinality",
}

// DiagnosticBranchInfo holds the information of a branch.
type DiagnosticBranchInfo struct {
	ID                string
	ConflictSet       []string
	IssuanceTimestamp time.Time
	BookingTime       time.Time
	LazyBooked        bool
	GradeOfFinality   gof.GradeOfFinality
}

func getDiagnosticConflictsInfo(branchID utxo.TransactionID) DiagnosticBranchInfo {
	conflictInfo := DiagnosticBranchInfo{
		ID: branchID.Base58(),
	}

	deps.Tangle.Ledger.ConflictDAG.Storage.CachedBranch(branchID).Consume(func(branch *conflictdag.Branch[utxo.TransactionID, utxo.OutputID]) {
		conflictInfo.GradeOfFinality, _ = deps.Tangle.Ledger.Utils.BranchGradeOfFinality(branch.ID())

		transactionID := branchID

		conflictInfo.ConflictSet = branch.ConflictIDs().Base58()

		deps.Tangle.Ledger.Storage.CachedTransaction(transactionID).Consume(func(transaction utxo.Transaction) {
			conflictInfo.IssuanceTimestamp = transaction.(*devnetvm.Transaction).Essence().Timestamp()
		})

		deps.Tangle.Ledger.Storage.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *ledger.TransactionMetadata) {
			conflictInfo.BookingTime = transactionMetadata.BookingTime()
		})
	})

	return conflictInfo
}

func (d DiagnosticBranchInfo) toCSV() (result string) {
	row := []string{
		d.ID,
		strings.Join(d.ConflictSet, ";"),
		fmt.Sprint(d.IssuanceTimestamp.UnixNano()),
		fmt.Sprint(d.BookingTime.UnixNano()),
		fmt.Sprint(d.LazyBooked),
		fmt.Sprint(d.GradeOfFinality),
	}

	result = strings.Join(row, ",")

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
