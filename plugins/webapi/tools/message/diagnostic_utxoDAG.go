package message

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/consensus/fcob"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
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

	deps.Tangle.Utils.WalkMessageID(func(messageID tangle.MessageID, walker *walker.Walker) {
		deps.Tangle.Utils.ComputeIfTransaction(messageID, func(transactionID ledgerstate.TransactionID) {
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
	}, tangle.MessageIDs{tangle.EmptyMessageID})

	c.Response().Flush()
}

// DiagnosticUTXODAGTableDescription holds the description of the diagnostic UTXODAG.
var DiagnosticUTXODAGTableDescription = []string{
	"ID",
	"IssuanceTime",
	"SolidTime",
	"OpinionFormedTime",
	"AccessManaPledgeID",
	"ConsensusManaPledgeID",
	"Inputs",
	"Outputs",
	"Attachments",
	"BranchID",
	"BranchLiked",
	"BranchMonotonicallyLiked",
	"Conflicting",
	"InclusionState",
	"Finalized",
	"LazyBooked",
	"Liked",
	"LoK",
	"FCOB1Time",
	"FCOB2Time",
}

// DiagnosticUTXODAGInfo holds the information of a UTXO.
type DiagnosticUTXODAGInfo struct {
	// transaction essence
	ID                    string
	IssuanceTimestamp     time.Time
	SolidTime             time.Time
	OpinionFormedTime     time.Time
	AccessManaPledgeID    string
	ConsensusManaPledgeID string
	Inputs                ledgerstate.Inputs
	Outputs               ledgerstate.Outputs
	// attachments
	Attachments []string
	// transaction metadata
	BranchID                 string
	BranchLiked              bool
	BranchMonotonicallyLiked bool
	Conflicting              bool
	InclusionState           string
	Finalized                bool
	LazyBooked               bool
	Liked                    bool
	LoK                      string
	FCOBTime1                time.Time
	FCOBTime2                time.Time
}

func getDiagnosticUTXODAGInfo(transactionID ledgerstate.TransactionID, messageID tangle.MessageID) DiagnosticUTXODAGInfo {
	txInfo := DiagnosticUTXODAGInfo{
		ID: transactionID.Base58(),
	}

	deps.Tangle.LedgerState.Transaction(transactionID).Consume(func(transaction *ledgerstate.Transaction) {
		txInfo.IssuanceTimestamp = transaction.Essence().Timestamp()
		txInfo.OpinionFormedTime = deps.ConsensusMechanism.(*fcob.ConsensusMechanism).OpinionFormedTime(messageID)
		txInfo.AccessManaPledgeID = base58.Encode(transaction.Essence().AccessPledgeID().Bytes())
		txInfo.ConsensusManaPledgeID = base58.Encode(transaction.Essence().ConsensusPledgeID().Bytes())
		txInfo.Inputs = transaction.Essence().Inputs()
		txInfo.Outputs = transaction.Essence().Outputs()
	})

	for _, messageID := range deps.Tangle.Storage.AttachmentMessageIDs(transactionID) {
		txInfo.Attachments = append(txInfo.Attachments, messageID.Base58())
	}

	deps.Tangle.LedgerState.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		txInfo.SolidTime = transactionMetadata.SolidificationTime()
		txInfo.BranchID = transactionMetadata.BranchID().String()

		deps.Tangle.LedgerState.BranchDAG.Branch(transactionMetadata.BranchID()).Consume(func(branch ledgerstate.Branch) {
			txInfo.BranchLiked = branch.Liked()
			txInfo.BranchMonotonicallyLiked = branch.MonotonicallyLiked()
		})

		txInfo.Conflicting = deps.Tangle.LedgerState.TransactionConflicting(transactionID)
		txInfo.Finalized = transactionMetadata.Finalized()
		txInfo.LazyBooked = transactionMetadata.LazyBooked()
		txInfo.InclusionState = deps.Tangle.LedgerState.BranchInclusionState(transactionMetadata.BranchID()).String()
		txInfo.Liked = deps.ConsensusMechanism.(*fcob.ConsensusMechanism).TransactionLiked(transactionID)
	})

	consensusMechanism := deps.Tangle.Options.ConsensusMechanism.(*fcob.ConsensusMechanism)
	if consensusMechanism != nil {
		consensusMechanism.Storage.Opinion(transactionID).Consume(func(opinion *fcob.Opinion) {
			txInfo.LoK = opinion.LevelOfKnowledge().String()
			txInfo.FCOBTime1 = opinion.FCOBTime1()
			txInfo.FCOBTime2 = opinion.FCOBTime2()
		})
	}

	return txInfo
}

func (d DiagnosticUTXODAGInfo) toCSV() (result string) {
	row := []string{
		d.ID,
		fmt.Sprint(d.IssuanceTimestamp.UnixNano()),
		fmt.Sprint(d.SolidTime.UnixNano()),
		fmt.Sprint(d.OpinionFormedTime.UnixNano()),
		d.AccessManaPledgeID,
		d.ConsensusManaPledgeID,
		strings.Join(d.Inputs.Strings(), ";"),
		strings.Join(d.Outputs.Strings(), ";"),
		strings.Join(d.Attachments, ";"),
		d.BranchID,
		fmt.Sprint(d.BranchLiked),
		fmt.Sprint(d.BranchMonotonicallyLiked),
		fmt.Sprint(d.Conflicting),
		d.InclusionState,
		fmt.Sprint(d.Finalized),
		fmt.Sprint(d.LazyBooked),
		fmt.Sprint(d.Liked),
		d.LoK,
		fmt.Sprint(d.FCOBTime1.UnixNano()),
		fmt.Sprint(d.FCOBTime2.UnixNano()),
	}

	result = strings.Join(row, ",")

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
