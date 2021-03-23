package message

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/identity"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
)

// DiagnosticMessagesHandler runs the diagnostic over the Tangle.
func DiagnosticMessagesHandler(c echo.Context) (err error) {
	runDiagnosticMessages(c)
	return
}

// DiagnosticMessagesOnlyFirstWeakReferencesHandler runs the diagnostic over the Tangle.
func DiagnosticMessagesOnlyFirstWeakReferencesHandler(c echo.Context) (err error) {
	runDiagnosticMessagesOnFirstWeakReferences(c)
	return
}

// DiagnosticMessagesRankHandler runs the diagnostic over the Tangle
// for messages with rank >= of the given rank parameter.
func DiagnosticMessagesRankHandler(c echo.Context) (err error) {
	rank, err := rankFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}
	runDiagnosticMessages(c, rank)
	return
}

// region Analysis code implementation /////////////////////////////////////////////////////////////////////////////////

func runDiagnosticMessages(c echo.Context, rank ...uint64) {
	// write Header and table description
	c.Response().Header().Set(echo.HeaderContentType, "text/csv")
	c.Response().WriteHeader(http.StatusOK)

	_, err := fmt.Fprintln(c.Response(), strings.Join(DiagnosticMessagesTableDescription, ","))
	if err != nil {
		panic(err)
	}

	startRank := uint64(0)

	if len(rank) > 0 {
		startRank = rank[0]
	}

	messagelayer.Tangle().Utils.WalkMessageID(func(messageID tangle.MessageID, walker *walker.Walker) {
		messageInfo := getDiagnosticMessageInfo(messageID)

		if messageInfo.Rank >= startRank {
			_, err = fmt.Fprintln(c.Response(), messageInfo.toCSV())
			if err != nil {
				panic(err)
			}
			c.Response().Flush()
		}

		messagelayer.Tangle().Storage.Approvers(messageID).Consume(func(approver *tangle.Approver) {
			walker.Push(approver.ApproverMessageID())
		})
	}, tangle.MessageIDs{tangle.EmptyMessageID})

	c.Response().Flush()
}

func runDiagnosticMessagesOnFirstWeakReferences(c echo.Context) {
	// write Header and table description
	c.Response().Header().Set(echo.HeaderContentType, "text/csv")
	c.Response().WriteHeader(http.StatusOK)

	_, err := fmt.Fprintln(c.Response(), strings.Join(DiagnosticMessagesTableDescription, ","))
	if err != nil {
		panic(err)
	}

	messagelayer.Tangle().Utils.WalkMessageID(func(messageID tangle.MessageID, walker *walker.Walker) {
		messageInfo := getDiagnosticMessageInfo(messageID)

		if len(messageInfo.WeakApprovers) > 0 {
			_, err = fmt.Fprintln(c.Response(), messageInfo.toCSV())
			if err != nil {
				panic(err)
			}
			c.Response().Flush()

			messagelayer.Tangle().Storage.Message(messageID).Consume(func(message *tangle.Message) {
				message.ForEachParent(func(parent tangle.Parent) {
					messageInfo = getDiagnosticMessageInfo(parent.ID)
					_, err = fmt.Fprintln(c.Response(), messageInfo.toCSV())
					if err != nil {
						panic(err)
					}
					c.Response().Flush()
				})
			})

			walker.StopWalk()
			return
		}

		messagelayer.Tangle().Storage.Approvers(messageID).Consume(func(approver *tangle.Approver) {
			if approver.Type() == tangle.StrongApprover {
				walker.Push(approver.ApproverMessageID())
			}
		})
	}, tangle.MessageIDs{tangle.EmptyMessageID})

	c.Response().Flush()
}

// DiagnosticMessagesTableDescription holds the description of the diagnostic messages.
var DiagnosticMessagesTableDescription = []string{
	"ID",
	"IssuerID",
	"IssuanceTime",
	"ArrivalTime",
	"SolidTime",
	"ScheduledTime",
	"BookedTime",
	"OpinionFormedTime",
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
	"PayloadType",
	"TransactionID",
}

// DiagnosticMessagesInfo holds the information of a message.
type DiagnosticMessagesInfo struct {
	ID                string
	IssuerID          string
	IssuanceTimestamp time.Time
	ArrivalTime       time.Time
	SolidTime         time.Time
	ScheduledTime     time.Time
	BookedTime        time.Time
	OpinionFormedTime time.Time
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
	PayloadType       string
	TransactionID     string
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
		msgInfo.PayloadType = message.Payload().Type().String()
		if message.Payload().Type() == ledgerstate.TransactionType {
			msgInfo.TransactionID = message.Payload().(*ledgerstate.Transaction).ID().Base58()
		}
	})

	branchID := ledgerstate.BranchID{}
	messagelayer.Tangle().Storage.MessageMetadata(messageID).Consume(func(metadata *tangle.MessageMetadata) {
		msgInfo.ArrivalTime = metadata.ReceivedTime()
		msgInfo.SolidTime = metadata.SolidificationTime()
		msgInfo.BranchID = metadata.BranchID().String()
		msgInfo.Scheduled = metadata.Scheduled()
		msgInfo.ScheduledTime = metadata.ScheduledTime()
		msgInfo.BookedTime = metadata.BookedTime()
		msgInfo.OpinionFormedTime = messagelayer.ConsensusMechanism().OpinionFormedTime(messageID)
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
		fmt.Sprint(d.ScheduledTime.UnixNano()),
		fmt.Sprint(d.BookedTime.UnixNano()),
		fmt.Sprint(d.OpinionFormedTime.UnixNano()),
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
		d.PayloadType,
		d.TransactionID,
	}

	result = strings.Join(row, ",")

	return
}

// rankFromContext determines the marker rank from the rank parameter in an echo.Context.
func rankFromContext(c echo.Context) (rank uint64, err error) {
	rank, err = strconv.ParseUint(c.Param("rank"), 10, 64)

	return rank, err
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
