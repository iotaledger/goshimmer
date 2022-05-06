package drng

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/walker"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// DiagnosticDRNGMessagesHandler runs the diagnostic over the Tangle.
func DiagnosticDRNGMessagesHandler(c echo.Context) (err error) {
	return runDiagnosticDRNGMessages(c)
}

// region DiagnosticDRNGMessages code implementation /////////////////////////////////////////////////////////////////////////////////

func runDiagnosticDRNGMessages(c echo.Context) (err error) {
	// write Header and table description
	c.Response().Header().Set(echo.HeaderContentType, "text/csv")
	c.Response().WriteHeader(http.StatusOK)

	csvWriter := csv.NewWriter(c.Response())
	if err := csvWriter.Write(DiagnosticDRNGMessagesTableDescription); err != nil {
		return errors.Errorf("failed to write table description row: %w", err)
	}

	var writeErr error
	deps.Tangle.Utils.WalkMessageID(func(messageID tangle.MessageID, walker *walker.Walker[tangle.MessageID]) {
		deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
			if message.Payload().Type() == drng.PayloadType {
				messageInfo := getDiagnosticDRNGMessageInfo(message)
				if messageInfo == nil {
					return
				}
				if err := csvWriter.Write(messageInfo.toCSVRow()); err != nil {
					writeErr = errors.Errorf("failed to write message diagnostic info row: %w", err)
					return
				}
			}
		})

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
	"dRNGPayloadType",
	"InstanceID",
	"Round",
	"PreviousSignature",
	"Signature",
	"DistributedPK",
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
	PayloadType       string
	InstanceID        uint32
	Round             uint64
	PreviousSignature string
	Signature         string
	DistributedPK     string
}

func getDiagnosticDRNGMessageInfo(message *tangle.Message) *DiagnosticDRNGMessagesInfo {
	msgInfo := &DiagnosticDRNGMessagesInfo{
		ID:                message.ID().Base58(),
		IssuanceTimestamp: message.IssuingTime(),
		IssuerID:          identity.NewID(message.IssuerPublicKey()).String(),
		IssuerPublicKey:   message.IssuerPublicKey().String(),
	}
	drngPayload := message.Payload().(*drng.CollectiveBeaconPayload)

	// parse as CollectiveBeaconType
	marshalUtil := marshalutil.New(drngPayload.Bytes())
	collectiveBeacon, err := drng.CollectiveBeaconPayloadFromMarshalUtil(marshalUtil)
	if err != nil {
		return nil
	}

	msgInfo.PayloadType = collectiveBeacon.Type().String()
	msgInfo.InstanceID = collectiveBeacon.InstanceID
	msgInfo.Round = collectiveBeacon.Round
	msgInfo.PreviousSignature = base58.Encode(collectiveBeacon.PrevSignature)
	msgInfo.Signature = base58.Encode(collectiveBeacon.Signature)
	msgInfo.DistributedPK = base58.Encode(collectiveBeacon.Dpk)

	deps.Tangle.Storage.MessageMetadata(message.ID()).Consume(func(metadata *tangle.MessageMetadata) {
		msgInfo.ArrivalTime = metadata.ReceivedTime()
		msgInfo.SolidTime = metadata.SolidificationTime()
		msgInfo.ScheduledTime = metadata.ScheduledTime()
		msgInfo.BookedTime = metadata.BookedTime()
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
		d.PayloadType,
		fmt.Sprint(d.InstanceID),
		fmt.Sprint(d.Round),
		d.PreviousSignature,
		d.Signature,
		d.DistributedPK,
	}

	return row
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
