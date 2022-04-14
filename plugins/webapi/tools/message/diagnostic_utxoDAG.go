package message

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/iotaledger/hive.go/generics/walker"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// DiagnosticUTXODAGHandler runs the diagnostic over the Tangle.
func DiagnosticUTXODAGHandler(c echo.Context) (err error) {
	runDiagnosticUTXODAG(c)
	return
}

// region DiagnosticUTXODAG code implementation /////////////////////////////////////////////////////////////////////////////////

func runDiagnosticUTXODAG(c echo.Context) {
	// write Header and table description
	c.Response().Header().Set(echo.HeaderContentType, "text/csv")
	c.Response().WriteHeader(http.StatusOK)

	_, err := fmt.Fprintln(c.Response(), strings.Join(DiagnosticUTXODAGTableDescription, ","))
	if err != nil {
		panic(err)
	}

	deps.Tangle.Utils.WalkMessageID(func(messageID tangle.MessageID, walker *walker.Walker[tangle.MessageID]) {
		deps.Tangle.Utils.ComputeIfTransaction(messageID, func(transactionID utxo.TransactionID) {
			transactionInfo := getDiagnosticUTXODAGInfo(transactionID, messageID)
			_, err = fmt.Fprintln(c.Response(), transactionInfo.toCSV())
			if err != nil {
				panic(err)
			}
			c.Response().Flush()
		})

		deps.Tangle.Storage.Approvers(messageID).Consume(func(approver *tangle.Approver) {
			walker.Push(approver.ApproverMessageID())
		})
	}, tangle.NewMessageIDs(tangle.EmptyMessageID))

	c.Response().Flush()
}

// DiagnosticUTXODAGTableDescription holds the description of the diagnostic UTXODAG.
var DiagnosticUTXODAGTableDescription = []string{
	"ID",
	"IssuanceTime",
	"SolidTime",
	"AccessManaPledgeID",
	"ConsensusManaPledgeID",
	"Inputs",
	"Outputs",
	"Attachments",
	"BranchID",
	"Conflicting",
	"LazyBooked",
	"GradeOfFinality",
	"GradeOfFinalityTime",
}

// DiagnosticUTXODAGInfo holds the information of a UTXO.
type DiagnosticUTXODAGInfo struct {
	// transaction essence
	ID                    string
	IssuanceTimestamp     time.Time
	SolidTime             time.Time
	AccessManaPledgeID    string
	ConsensusManaPledgeID string
	Inputs                devnetvm.Inputs
	Outputs               devnetvm.Outputs
	// attachments
	Attachments []string
	// transaction metadata
	BranchIDs           []string
	Conflicting         bool
	LazyBooked          bool
	GradeOfFinality     gof.GradeOfFinality
	GradeOfFinalityTime time.Time
}

func getDiagnosticUTXODAGInfo(transactionID utxo.TransactionID, messageID tangle.MessageID) DiagnosticUTXODAGInfo {
	txInfo := DiagnosticUTXODAGInfo{
		ID: transactionID.Base58(),
	}

	deps.Tangle.Ledger.Storage.CachedTransaction(transactionID).Consume(func(transaction *ledger.Transaction) {
		transactionEssence := transaction.Transaction.(*devnetvm.Transaction).Essence()

		txInfo.IssuanceTimestamp = transactionEssence.Timestamp()
		txInfo.AccessManaPledgeID = base58.Encode(transactionEssence.AccessPledgeID().Bytes())
		txInfo.ConsensusManaPledgeID = base58.Encode(transactionEssence.ConsensusPledgeID().Bytes())
		txInfo.Inputs = transactionEssence.Inputs()
		txInfo.Outputs = transactionEssence.Outputs()
	})

	for messageID := range deps.Tangle.Storage.AttachmentMessageIDs(transactionID) {
		txInfo.Attachments = append(txInfo.Attachments, messageID.Base58())
	}

	deps.Tangle.Ledger.Storage.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *ledger.TransactionMetadata) {
		txInfo.SolidTime = transactionMetadata.SolidificationTime()
		txInfo.BranchIDs = transactionMetadata.BranchIDs().Base58()

		txInfo.Conflicting = deps.Tangle.Ledger.Storage.CachedTransactionConflicting(transactionID)
		txInfo.LazyBooked = transactionMetadata.LazyBooked()
		txInfo.GradeOfFinality = transactionMetadata.GradeOfFinality()
		txInfo.GradeOfFinalityTime = transactionMetadata.GradeOfFinalityTime()
	})

	return txInfo
}

func (d DiagnosticUTXODAGInfo) toCSV() (result string) {
	row := []string{
		d.ID,
		fmt.Sprint(d.IssuanceTimestamp.UnixNano()),
		fmt.Sprint(d.SolidTime.UnixNano()),
		d.AccessManaPledgeID,
		d.ConsensusManaPledgeID,
		strings.Join(d.Inputs.Strings(), ";"),
		strings.Join(d.Outputs.Strings(), ";"),
		strings.Join(d.Attachments, ";"),
		strings.Join(d.BranchIDs, ";"),
		fmt.Sprint(d.Conflicting),
		fmt.Sprint(d.LazyBooked),
		fmt.Sprint(d.GradeOfFinality),
		fmt.Sprint(d.GradeOfFinalityTime.UnixNano()),
	}

	result = strings.Join(row, ",")

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
