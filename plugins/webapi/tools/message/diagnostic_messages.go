package message

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/xerrors"

	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/identity"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/consensus/fcob"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
)

// DiagnosticMessagesHandler runs the diagnostic over the Tangle.
func DiagnosticMessagesHandler(c echo.Context) (err error) {
	return runDiagnosticMessages(c)
}

// DiagnosticMessagesOnlyFirstWeakReferencesHandler runs the diagnostic over the Tangle.
func DiagnosticMessagesOnlyFirstWeakReferencesHandler(c echo.Context) (err error) {
	return runDiagnosticMessagesOnFirstWeakReferences(c)
}

// DiagnosticMessagesRankHandler runs the diagnostic over the Tangle
// for messages with rank >= of the given rank parameter.
func DiagnosticMessagesRankHandler(c echo.Context) (err error) {
	rank, err := rankFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}
	return runDiagnosticMessages(c, rank)
}

// region DiagnosticMessages code implementation /////////////////////////////////////////////////////////////////////////////////

func runDiagnosticMessages(c echo.Context, rank ...uint64) (err error) {
	// write Header and table description
	c.Response().Header().Set(echo.HeaderContentType, "text/csv")
	c.Response().WriteHeader(http.StatusOK)

	csvWriter := csv.NewWriter(c.Response())
	if err := csvWriter.Write(DiagnosticMessagesTableDescription); err != nil {
		return xerrors.Errorf("failed to write table description row: %w", err)
	}

	startRank := uint64(0)

	if len(rank) > 0 {
		startRank = rank[0]
	}
	var writeErr error
	messagelayer.Tangle().Utils.WalkMessageID(func(messageID tangle.MessageID, walker *walker.Walker) {
		messageInfo := getDiagnosticMessageInfo(messageID)

		if messageInfo.Rank >= startRank {
			if err := csvWriter.Write(messageInfo.toCSVRow()); err != nil {
				writeErr = xerrors.Errorf("failed to write message diagnostic info row: %w", err)
				return
			}
		}

		messagelayer.Tangle().Storage.Approvers(messageID).Consume(func(approver *tangle.Approver) {
			walker.Push(approver.ApproverMessageID())
		})
	}, tangle.MessageIDs{tangle.EmptyMessageID})
	if writeErr != nil {
		return writeErr
	}
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return xerrors.Errorf("csv writer failed after flush: %w", err)
	}

	return nil
}

func runDiagnosticMessagesOnFirstWeakReferences(c echo.Context) (err error) {
	// write Header and table description
	c.Response().Header().Set(echo.HeaderContentType, "text/csv")
	c.Response().WriteHeader(http.StatusOK)

	csvWriter := csv.NewWriter(c.Response())
	if err := csvWriter.Write(DiagnosticMessagesTableDescription); err != nil {
		return xerrors.Errorf("failed to write table description row: %w", err)
	}
	var writeErr error
	messagelayer.Tangle().Utils.WalkMessageID(func(messageID tangle.MessageID, walker *walker.Walker) {
		messageInfo := getDiagnosticMessageInfo(messageID)

		if len(messageInfo.WeakApprovers) > 0 {
			if err := csvWriter.Write(messageInfo.toCSVRow()); err != nil {
				writeErr = xerrors.Errorf("failed to write message diagnostic info row: %w", err)
				return
			}

			messagelayer.Tangle().Storage.Message(messageID).Consume(func(message *tangle.Message) {
				message.ForEachParent(func(parent tangle.Parent) {
					parentMessageInfo := getDiagnosticMessageInfo(parent.ID)
					if err := csvWriter.Write(parentMessageInfo.toCSVRow()); err != nil {
						writeErr = xerrors.Errorf("failed to write parent message diagnostic info row: %w", err)
						return
					}
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
	if writeErr != nil {
		return writeErr
	}

	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return xerrors.Errorf("csv writer failed after flush: %w", err)
	}

	return nil
}

// DiagnosticMessagesTableDescription holds the description of the diagnostic messages.
var DiagnosticMessagesTableDescription = []string{
	"ID",
	"IssuerID",
	"IssuerPublicKey",
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
	"PayloadOpinionFormed",
	"TimestampOpinionFormed",
	"MessageOpinionFormed",
	"MessageOpinionTriggered",
	"TimestampOpinion",
	"TimestampLoK",
}

// DiagnosticMessagesInfo holds the information of a message.
type DiagnosticMessagesInfo struct {
	ID                string
	IssuerID          string
	IssuerPublicKey   string
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
	// Consensus information
	PayloadOpinionFormed    bool
	TimestampOpinionFormed  bool
	MessageOpinionFormed    bool
	MessageOpinionTriggered bool
	TimestampOpinion        string
	TimestampLoK            string
}

func getDiagnosticMessageInfo(messageID tangle.MessageID) *DiagnosticMessagesInfo {
	msgInfo := &DiagnosticMessagesInfo{
		ID: messageID.Base58(),
	}

	messagelayer.Tangle().Storage.Message(messageID).Consume(func(message *tangle.Message) {
		msgInfo.IssuanceTimestamp = message.IssuingTime()
		msgInfo.IssuerID = identity.NewID(message.IssuerPublicKey()).String()
		msgInfo.IssuerPublicKey = message.IssuerPublicKey().String()
		msgInfo.StrongParents = message.StrongParents()
		msgInfo.WeakParents = message.WeakParents()
		msgInfo.PayloadType = message.Payload().Type().String()
		if message.Payload().Type() == ledgerstate.TransactionType {
			msgInfo.TransactionID = message.Payload().(*ledgerstate.Transaction).ID().Base58()
		}
	})

	branchID, err := messagelayer.Tangle().Booker.MessageBranchID(messageID)
	if err != nil {
		branchID = ledgerstate.BranchID{}
	}
	messagelayer.Tangle().Storage.MessageMetadata(messageID).Consume(func(metadata *tangle.MessageMetadata) {
		msgInfo.ArrivalTime = metadata.ReceivedTime()
		msgInfo.SolidTime = metadata.SolidificationTime()
		msgInfo.BranchID = branchID.String()
		msgInfo.Scheduled = metadata.Scheduled()
		msgInfo.ScheduledTime = metadata.ScheduledTime()
		msgInfo.BookedTime = metadata.BookedTime()
		msgInfo.OpinionFormedTime = messagelayer.ConsensusMechanism().OpinionFormedTime(messageID)
		msgInfo.Booked = metadata.IsBooked()
		msgInfo.Eligible = metadata.IsEligible()
		msgInfo.Invalid = metadata.IsInvalid()
		if metadata.StructureDetails() != nil {
			msgInfo.Rank = metadata.StructureDetails().Rank
			msgInfo.IsPastMarker = metadata.StructureDetails().IsPastMarker
			msgInfo.PastMarkers = metadata.StructureDetails().PastMarkers.SequenceToString()
			msgInfo.PMHI = uint64(metadata.StructureDetails().PastMarkers.HighestIndex())
			msgInfo.PMLI = uint64(metadata.StructureDetails().PastMarkers.LowestIndex())
			msgInfo.FutureMarkers = metadata.StructureDetails().FutureMarkers.SequenceToString()
			msgInfo.FMHI = uint64(metadata.StructureDetails().FutureMarkers.HighestIndex())
			msgInfo.FMLI = uint64(metadata.StructureDetails().FutureMarkers.LowestIndex())
		}
	}, false)

	msgInfo.StrongApprovers = messagelayer.Tangle().Utils.ApprovingMessageIDs(messageID, tangle.StrongApprover)
	msgInfo.WeakApprovers = messagelayer.Tangle().Utils.ApprovingMessageIDs(messageID, tangle.WeakApprover)

	msgInfo.InclusionState = messagelayer.Tangle().LedgerState.BranchInclusionState(branchID).String()

	// add consensus information
	consensusMechanism := messagelayer.Tangle().Options.ConsensusMechanism.(*fcob.ConsensusMechanism)
	if consensusMechanism != nil {
		consensusMechanism.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *fcob.MessageMetadata) {
			msgInfo.PayloadOpinionFormed = messageMetadata.PayloadOpinionFormed()
			msgInfo.TimestampOpinionFormed = messageMetadata.TimestampOpinionFormed()
			msgInfo.MessageOpinionFormed = messageMetadata.MessageOpinionFormed()
			msgInfo.MessageOpinionTriggered = messageMetadata.MessageOpinionTriggered()
		})

		consensusMechanism.Storage.TimestampOpinion(messageID).Consume(func(timestampOpinion *fcob.TimestampOpinion) {
			msgInfo.TimestampOpinion = timestampOpinion.Value.String()
			msgInfo.TimestampLoK = timestampOpinion.LoK.String()
		})
	}

	return msgInfo
}

func (d *DiagnosticMessagesInfo) toCSVRow() (row []string) {
	row = []string{
		d.ID,
		d.IssuerID,
		d.IssuerPublicKey,
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
		fmt.Sprint(d.PayloadOpinionFormed),
		fmt.Sprint(d.TimestampOpinionFormed),
		fmt.Sprint(d.MessageOpinionFormed),
		fmt.Sprint(d.MessageOpinionTriggered),
		d.TimestampOpinion,
		d.TimestampLoK,
	}

	return row
}

// rankFromContext determines the marker rank from the rank parameter in an echo.Context.
func rankFromContext(c echo.Context) (rank uint64, err error) {
	rank, err = strconv.ParseUint(c.Param("rank"), 10, 64)

	return rank, err
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
