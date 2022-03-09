package message

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/walker"
	"github.com/iotaledger/hive.go/identity"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
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
		return errors.Errorf("failed to write table description row: %w", err)
	}

	startRank := uint64(0)

	if len(rank) > 0 {
		startRank = rank[0]
	}
	var writeErr error
	deps.Tangle.Utils.WalkMessageID(func(messageID tangle.MessageID, walker *walker.Walker[tangle.MessageID]) {
		messageInfo := getDiagnosticMessageInfo(messageID)

		if messageInfo.Rank >= startRank {
			if err := csvWriter.Write(messageInfo.toCSVRow()); err != nil {
				writeErr = errors.Errorf("failed to write message diagnostic info row: %w", err)
				return
			}
		}

		deps.Tangle.Storage.Approvers(messageID).Consume(func(approver *tangle.Approver) {
			walker.Push(approver.ApproverMessageID())
		})
	}, tangle.NewMessageIDs(tangle.EmptyMessageID))
	if writeErr != nil {
		return writeErr
	}
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return errors.Errorf("csv writer failed after flush: %w", err)
	}

	return nil
}

func runDiagnosticMessagesOnFirstWeakReferences(c echo.Context) (err error) {
	// write Header and table description
	c.Response().Header().Set(echo.HeaderContentType, "text/csv")
	c.Response().WriteHeader(http.StatusOK)

	csvWriter := csv.NewWriter(c.Response())
	if err := csvWriter.Write(DiagnosticMessagesTableDescription); err != nil {
		return errors.Errorf("failed to write table description row: %w", err)
	}
	var writeErr error
	deps.Tangle.Utils.WalkMessageID(func(messageID tangle.MessageID, walker *walker.Walker[tangle.MessageID]) {
		messageInfo := getDiagnosticMessageInfo(messageID)

		if len(messageInfo.WeakApprovers) > 0 {
			if err := csvWriter.Write(messageInfo.toCSVRow()); err != nil {
				writeErr = errors.Errorf("failed to write message diagnostic info row: %w", err)
				return
			}

			deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
				message.ForEachParent(func(parent tangle.Parent) {
					parentMessageInfo := getDiagnosticMessageInfo(parent.ID)
					if err := csvWriter.Write(parentMessageInfo.toCSVRow()); err != nil {
						writeErr = errors.Errorf("failed to write parent message diagnostic info row: %w", err)
						return
					}
				})
			})

			walker.StopWalk()
			return
		}

		deps.Tangle.Storage.Approvers(messageID).Consume(func(approver *tangle.Approver) {
			if approver.Type() == tangle.StrongApprover {
				walker.Push(approver.ApproverMessageID())
			}
		})
	}, tangle.NewMessageIDs(tangle.EmptyMessageID))
	if writeErr != nil {
		return writeErr
	}

	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return errors.Errorf("csv writer failed after flush: %w", err)
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
	"GradeOfFinality",
	"GradeOfFinalityTime",
	"StrongParents",
	"WeakParents",
	"DislikeParents",
	"LikeParents",
	"StrongApprovers",
	"WeakApprovers",
	"ShallowLikeApprovers",
	"ShallowDislikeApprovers",
	"BranchID",
	"Scheduled",
	"Booked",
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
	ID                      string
	IssuerID                string
	IssuerPublicKey         string
	IssuanceTimestamp       time.Time
	ArrivalTime             time.Time
	SolidTime               time.Time
	ScheduledTime           time.Time
	BookedTime              time.Time
	GradeOfFinality         gof.GradeOfFinality
	GradeOfFinalityTime     time.Time
	StrongParents           []string
	WeakParents             []string
	ShallowDislikeParents   []string
	ShallowLikeParents      []string
	StrongApprovers         []string
	WeakApprovers           []string
	ShallowLikeApprovers    []string
	ShallowDislikeApprovers []string
	BranchIDs               []string
	AddedBranchIDs          []string
	SubtractedBranchIDs     []string
	Scheduled               bool
	Booked                  bool
	ObjectivelyInvalid      bool
	Rank                    uint64
	IsPastMarker            bool
	PastMarkers             string // PastMarkers
	PMHI                    uint64 // PastMarkers Highest Index
	PMLI                    uint64 // PastMarkers Lowest Index
	FutureMarkers           string // FutureMarkers
	FMHI                    uint64 // FutureMarkers Highest Index
	FMLI                    uint64 // FutureMarkers Lowest Index
	PayloadType             string
	TransactionID           string
}

func getDiagnosticMessageInfo(messageID tangle.MessageID) *DiagnosticMessagesInfo {
	msgInfo := &DiagnosticMessagesInfo{
		ID: messageID.Base58(),
	}

	deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
		msgInfo.IssuanceTimestamp = message.IssuingTime()
		msgInfo.IssuerID = identity.NewID(message.IssuerPublicKey()).String()
		msgInfo.IssuerPublicKey = message.IssuerPublicKey().String()
		msgInfo.StrongParents = message.ParentsByType(tangle.StrongParentType).Base58()
		msgInfo.WeakParents = message.ParentsByType(tangle.WeakParentType).Base58()
		msgInfo.ShallowDislikeParents = message.ParentsByType(tangle.ShallowDislikeParentType).Base58()
		msgInfo.ShallowLikeParents = message.ParentsByType(tangle.ShallowLikeParentType).Base58()
		msgInfo.PayloadType = message.Payload().Type().String()
		if message.Payload().Type() == ledgerstate.TransactionType {
			msgInfo.TransactionID = message.Payload().(*ledgerstate.Transaction).ID().Base58()
		}
	})

	branchIDs, _ := deps.Tangle.Booker.MessageBranchIDs(messageID)

	deps.Tangle.Storage.MessageMetadata(messageID).Consume(func(metadata *tangle.MessageMetadata) {
		msgInfo.ArrivalTime = metadata.ReceivedTime()
		msgInfo.SolidTime = metadata.SolidificationTime()
		msgInfo.BranchIDs = branchIDs.Base58()
		msgInfo.Scheduled = metadata.Scheduled()
		msgInfo.ScheduledTime = metadata.ScheduledTime()
		msgInfo.BookedTime = metadata.BookedTime()
		msgInfo.GradeOfFinality = metadata.GradeOfFinality()
		msgInfo.GradeOfFinalityTime = metadata.GradeOfFinalityTime()
		msgInfo.Booked = metadata.IsBooked()
		msgInfo.ObjectivelyInvalid = metadata.IsObjectivelyInvalid()
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

	msgInfo.StrongApprovers = deps.Tangle.Utils.ApprovingMessageIDs(messageID, tangle.StrongApprover).Base58()
	msgInfo.WeakApprovers = deps.Tangle.Utils.ApprovingMessageIDs(messageID, tangle.WeakApprover).Base58()
	msgInfo.ShallowLikeApprovers = deps.Tangle.Utils.ApprovingMessageIDs(messageID, tangle.ShallowLikeApprover).Base58()
	msgInfo.ShallowDislikeApprovers = deps.Tangle.Utils.ApprovingMessageIDs(messageID, tangle.ShallowDislikeApprover).Base58()

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
		fmt.Sprint(d.GradeOfFinality.String()),
		fmt.Sprint(d.GradeOfFinalityTime.UnixNano()),
		strings.Join(d.StrongParents, ";"),
		strings.Join(d.WeakParents, ";"),
		strings.Join(d.ShallowDislikeParents, ";"),
		strings.Join(d.ShallowLikeParents, ";"),
		strings.Join(d.StrongApprovers, ";"),
		strings.Join(d.WeakApprovers, ";"),
		strings.Join(d.ShallowLikeApprovers, ";"),
		strings.Join(d.ShallowDislikeParents, ";"),
		strings.Join(d.BranchIDs, ";"),
		fmt.Sprint(d.Scheduled),
		fmt.Sprint(d.Booked),
		fmt.Sprint(d.ObjectivelyInvalid),
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

	return row
}

// rankFromContext determines the marker rank from the rank parameter in an echo.Context.
func rankFromContext(c echo.Context) (rank uint64, err error) {
	rank, err = strconv.ParseUint(c.Param("rank"), 10, 64)

	return rank, err
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
