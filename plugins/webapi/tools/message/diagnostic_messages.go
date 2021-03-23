package message

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/identity"
	"github.com/labstack/echo"
)

// DiagnosticMessagesHandler runs the diagnostic over the Tangle.
func DiagnosticMessagesHandler(c echo.Context) (err error) {
	runDiagnosticMessages(c)
	return
}

// region Analysis code implementation /////////////////////////////////////////////////////////////////////////////////

func runDiagnosticMessages(c echo.Context) {
	// write Header and table description
	c.Response().Header().Set(echo.HeaderContentType, "text/csv")
	c.Response().WriteHeader(http.StatusOK)

	_, err := fmt.Fprintln(c.Response(), strings.Join(DiagnosticMessagesTableDescription, ","))
	if err != nil {
		panic(err)
	}

	messagelayer.Tangle().Utils.WalkMessageID(func(messageID tangle.MessageID, walker *walker.Walker) {
		messageInfo := getDiagnosticMessageInfo(messageID)
		_, err = fmt.Fprintln(c.Response(), messageInfo.toCSV())
		if err != nil {
			panic(err)
		}
		c.Response().Flush()

		messagelayer.Tangle().Storage.Approvers(messageID).Consume(func(approver *tangle.Approver) {
			walker.Push(approver.ApproverMessageID())
		})
	}, tangle.MessageIDs{tangle.EmptyMessageID})

	c.Response().Flush()
	return
}

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
	"Scheduled",
	"Booked",
	"Eligible",
	"Invalid",
	"Rank",
	"IsPastMarker",
	"PastMarkers",
	"PMHI",
	"PMLI",
	"FutureMarkers",
	"FMHI",
	"FMLI",
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
	Scheduled         bool
	Booked            bool
	Eligible          bool
	Invalid           bool
	Rank              uint64
	IsPastMarker      bool
	PastMarkers       string // PastMarkers
	PMHI              uint64 // PastMarkers Highest Index
	PMLI              uint64 // PastMarkers Lowest Index
	FutureMarkers     string // FutureMarkers
	FMHI              uint64 // FutureMarkers Highest Index
	FMLI              uint64 // FutureMarkers Lowest Index
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
		msgInfo.Scheduled = metadata.Scheduled()
		msgInfo.Booked = metadata.IsBooked()
		msgInfo.Eligible = metadata.IsEligible()
		msgInfo.Invalid = metadata.IsInvalid()
		msgInfo.Rank = metadata.StructureDetails().Rank
		msgInfo.IsPastMarker = metadata.StructureDetails().IsPastMarker
		msgInfo.PastMarkers = metadata.StructureDetails().PastMarkers.SequenceToString()
		msgInfo.PMHI = uint64(metadata.StructureDetails().PastMarkers.HighestIndex())
		msgInfo.PMLI = uint64(metadata.StructureDetails().PastMarkers.LowestIndex())
		msgInfo.FutureMarkers = metadata.StructureDetails().FutureMarkers.SequenceToString()
		msgInfo.FMHI = uint64(metadata.StructureDetails().FutureMarkers.HighestIndex())
		msgInfo.FMLI = uint64(metadata.StructureDetails().FutureMarkers.LowestIndex())

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
		strings.Join(d.StrongParents.ToStrings(), ";"),
		strings.Join(d.WeakParents.ToStrings(), ";"),
		strings.Join(d.StrongApprovers.ToStrings(), ";"),
		strings.Join(d.WeakApprovers.ToStrings(), ";"),
		d.BranchID,
		d.InclusionState,
		fmt.Sprint(d.Scheduled),
		fmt.Sprint(d.Booked),
		fmt.Sprint(d.Eligible),
		fmt.Sprint(d.Invalid),
		fmt.Sprint(d.Rank),
		fmt.Sprint(d.IsPastMarker),
		d.PastMarkers,
		fmt.Sprint(d.PMHI),
		fmt.Sprint(d.PMLI),
		d.FutureMarkers,
		fmt.Sprint(d.FMHI),
		fmt.Sprint(d.FMLI),
	}

	result = strings.Join(row, ",")

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
