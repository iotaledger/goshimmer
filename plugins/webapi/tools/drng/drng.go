package drng

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/identity"
	"github.com/labstack/echo"
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

// DiagnosticDRNGMessagesHandler runs the diagnostic over the Tangle.
func DiagnosticDRNGMessagesHandler(c echo.Context) (err error) {
	return runDiagnosticDRNGMessages(c)
}

// region DiagnosticDRNGMessages code implementation /////////////////////////////////////////////////////////////////////////////////

func runDiagnosticDRNGMessages(c echo.Context, rank ...uint64) (err error) {
	// write Header and table description
	c.Response().Header().Set(echo.HeaderContentType, "text/csv")
	c.Response().WriteHeader(http.StatusOK)

	csvWriter := csv.NewWriter(c.Response())
	if err := csvWriter.Write(DiagnosticDRNGMessagesTableDescription); err != nil {
		return xerrors.Errorf("failed to write table description row: %w", err)
	}

	var writeErr error
	messagelayer.Tangle().Utils.WalkMessage(func(message *tangle.Message, walker *walker.Walker) {
		if message.Payload().Type() == drng.PayloadType {
			messageInfo := getDiagnosticDRNGMessageInfo(message.ID())

			if err := csvWriter.Write(messageInfo.toCSVRow()); err != nil {
				writeErr = xerrors.Errorf("failed to write message diagnostic info row: %w", err)
				return
			}
		}

		messagelayer.Tangle().Storage.Approvers(message.ID()).Consume(func(approver *tangle.Approver) {
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

// DiagnosticDRNGMessagesTableDescription holds the description of the diagnostic dRNG messages.
var DiagnosticDRNGMessagesTableDescription = []string{
	"ID",
	"IssuerID",
	"IssuerPublicKey",
	"IssuanceTime",
	"ArrivalTime",
	"SolidTime",
	"ScheduledTime",
	"BookedTime",
	"OpinionFormedTime",
}

// DiagnosticDRNGMessagesInfo holds the information of a dRNG message.
type DiagnosticDRNGMessagesInfo struct {
	ID                string
	IssuerID          string
	IssuerPublicKey   string
	IssuanceTimestamp time.Time
	ArrivalTime       time.Time
	SolidTime         time.Time
	ScheduledTime     time.Time
	BookedTime        time.Time
	OpinionFormedTime time.Time
}

func getDiagnosticDRNGMessageInfo(messageID tangle.MessageID) *DiagnosticDRNGMessagesInfo {
	msgInfo := &DiagnosticDRNGMessagesInfo{
		ID: messageID.String(),
	}

	messagelayer.Tangle().Storage.Message(messageID).Consume(func(message *tangle.Message) {
		msgInfo.IssuanceTimestamp = message.IssuingTime()
		msgInfo.IssuerID = identity.NewID(message.IssuerPublicKey()).String()
		msgInfo.IssuerPublicKey = message.IssuerPublicKey().String()
	})

	messagelayer.Tangle().Storage.MessageMetadata(messageID).Consume(func(metadata *tangle.MessageMetadata) {
		msgInfo.ArrivalTime = metadata.ReceivedTime()
		msgInfo.SolidTime = metadata.SolidificationTime()
		msgInfo.ScheduledTime = metadata.ScheduledTime()
		msgInfo.BookedTime = metadata.BookedTime()
		msgInfo.OpinionFormedTime = messagelayer.ConsensusMechanism().OpinionFormedTime(messageID)
	}, false)

	return msgInfo
}

func (d *DiagnosticDRNGMessagesInfo) toCSVRow() (row []string) {
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
	}

	return row
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
