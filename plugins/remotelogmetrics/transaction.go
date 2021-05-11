package remotelogmetrics

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
)

type transactionMetricsLogger struct {
	Type               string    `json:"type" bson:"type"`
	MessageID          string    `json:"messageID" bson:"messageID"`
	TransactionID      string    `json:"transactionID" bson:"transactionID"`
	IssuedTimestamp    time.Time `json:"issuedTimestamp" bson:"issuedTimestamp"`
	SolidTimestamp     time.Time `json:"solidTimestamp" bson:"solidTimestamp"`
	ScheduledTimestamp time.Time `json:"scheduledTimestamp" bson:"scheduledTimestamp"`
	BookedTimestamp    time.Time `json:"bookedTimestamp" bson:"bookedTimestamp"`
	ConfirmedTimestamp time.Time `json:"confirmedTimestamp" bson:"confirmedTimestamp"`
}

func newTransactionMetricsLogger() *transactionMetricsLogger {
	return &transactionMetricsLogger{}
}

func (ml *transactionMetricsLogger) onTransactionConfirmed(transactionID ledgerstate.TransactionID) {
	record := &transactionMetricsLogger{
		Type:          "transaction",
		TransactionID: transactionID.Base58(),
	}

	messageIDs := messagelayer.Tangle().Storage.AttachmentMessageIDs(transactionID)
	if len(messageIDs) == 0 {
		return
	}
	messageID := messageIDs[0]
	messagelayer.Tangle().Storage.Message(messageID).Consume(func(message *tangle.Message) {
		record.MessageID = messageID.Base58()
		record.IssuedTimestamp = message.IssuingTime()
	})
	messagelayer.Tangle().Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
		record.ScheduledTimestamp = messageMetadata.ScheduledTime()
		record.BookedTimestamp = messageMetadata.BookedTime()
		record.ConfirmedTimestamp = messageMetadata.FinalizedTime()
	})

	messagelayer.Tangle().LedgerState.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		record.SolidTimestamp = transactionMetadata.SolidificationTime()
	})

	if err := remotelog.RemoteLogger().Send(record); err != nil {
		plugin.Logger().Errorw("Failed to send Transaction record", "err", err)
	}
}
