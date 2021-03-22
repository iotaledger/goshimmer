package message

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/identity"
	"github.com/labstack/echo"
)

// DiagnosticMessagesHandler runs the diagnostic over the Tangle.
func DiagnosticMessagesHandler(c echo.Context) error {
	res := &DiagnosticMessagesResponse{}
	res.Err = runDiagnosticMessages()
	if res.Err != nil {
		return c.JSON(http.StatusInternalServerError, res)
	}
	return c.JSON(http.StatusOK, res)
}

// DiagnosticMessagesResponse is the HTTP response.
type DiagnosticMessagesResponse struct {
	Result string `json:"result,omitempty"`
	Err    error  `json:"error,omitempty"`
}

// region Analysis code implementation /////////////////////////////////////////////////////////////////////////////////

// func firstApprovalAnalysis(nodeID string, filePath string) (err error) {
// 	// write TableDescription
// 	if err := w.Write(TableDescription); err != nil {
// 		return err
// 	}

// 	messagelayer.Tangle().Utils.WalkMessageID(func(msgID tangle.MessageID, walker *walker.Walker) {
// 		approverInfo, err := firstApprovers(msgID)
// 		// firstApprovers returns an error when the msgID is a tip, thus
// 		// we want to stop the computation but continue with the future cone iteration.
// 		if err != nil {
// 			return
// 		}

// 		msgApproval := MsgApproval{
// 			NodeID:                  nodeID,
// 			Msg:                     info(msgID),
// 			FirstApproverByIssuance: approverInfo[byIssuance],
// 			FirstApproverByArrival:  approverInfo[byArrival],
// 			FirstApproverBySolid:    approverInfo[bySolid],
// 		}

// 		// write msgApproval to file
// 		if err = w.Write(msgApproval.toCSV()); err != nil {
// 			return
// 		}
// 		w.Flush()
// 		if err = w.Error(); err != nil {
// 			return
// 		}

// 		messagelayer.Tangle().Storage.Approvers(msgID).Consume(func(approver *tangle.Approver) {
// 			walker.Push(approver.ApproverMessageID())
// 		})
// 	}, tangle.MessageIDs{tangle.EmptyMessageID})

// 	return
// }

// DiagnosticMessagesTableDescription holds the description of the diagnostic messages.
var DiagnosticMessagesTableDescription = []string{
	"ID",
	"IssuerID",
	"IssuanceTime",
	"ArrivalTime",
	"SolidTime",
	"StrongParents",
	"WeakParents",
	"StrongApprovers",
	"WeakApprovers",
	"BranchID",
	"InclusionState",
}

// DiagnosticMessagesInfo holds the information of a message.
type DiagnosticMessagesInfo struct {
	ID                string
	IssuerID          string
	IssuanceTimestamp time.Time
	ArrivalTime       time.Time
	SolidTime         time.Time
	StrongParents     tangle.MessageIDs
	WeakParents       tangle.MessageIDs
	StrongApprovers   tangle.MessageIDs
	WeakApprovers     tangle.MessageIDs
	BranchID          string
	InclusionState    string
}

func getDiagnosticMessageInfo(messageID tangle.MessageID) DiagnosticMessagesInfo {
	msgInfo := DiagnosticMessagesInfo{
		ID: messageID.String(),
	}

	messagelayer.Tangle().Storage.Message(messageID).Consume(func(message *tangle.Message) {
		msgInfo.IssuanceTimestamp = message.IssuingTime()
		msgInfo.IssuerID = identity.NewID(message.IssuerPublicKey()).String()
		msgInfo.StrongParents = message.StrongParents()
		msgInfo.WeakParents = message.WeakParents()
	})

	branchID := ledgerstate.BranchID{}
	messagelayer.Tangle().Storage.MessageMetadata(messageID).Consume(func(metadata *tangle.MessageMetadata) {
		msgInfo.ArrivalTime = metadata.ReceivedTime()
		msgInfo.SolidTime = metadata.SolidificationTime()
		msgInfo.BranchID = metadata.BranchID().String()
		branchID = metadata.BranchID()
	}, false)

	msgInfo.StrongApprovers = messagelayer.Tangle().Utils.ApprovingMessageIDs(messageID, tangle.StrongApprover)
	msgInfo.WeakApprovers = messagelayer.Tangle().Utils.ApprovingMessageIDs(messageID, tangle.WeakApprover)

	msgInfo.InclusionState = messagelayer.Tangle().LedgerState.BranchInclusionState(branchID).String()

	return msgInfo
}

func (d DiagnosticMessagesInfo) toCSV() (result string) {
	row := []string{
		d.ID,
		d.IssuerID,
		fmt.Sprint(d.IssuanceTimestamp.UnixNano()),
		fmt.Sprint(d.ArrivalTime.UnixNano()),
		fmt.Sprint(d.SolidTime.UnixNano()),
		strings.Join(d.StrongParents.ToStrings(), ":"),
		strings.Join(d.WeakParents.ToStrings(), ":"),
		strings.Join(d.StrongApprovers.ToStrings(), ":"),
		strings.Join(d.WeakApprovers.ToStrings(), ":"),
		d.BranchID,
		d.InclusionState,
	}

	result = strings.Join(row, ",")

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
