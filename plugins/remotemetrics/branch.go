package remotemetrics

import (
	"github.com/iotaledger/hive.go/events"
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/remotemetrics"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
)

func onBranchConfirmed(branchID ledgerstate.BranchID, newLevel int, transition events.ThresholdEventTransition) {
	if !messagelayer.Tangle().Synced() {
		return
	}
	if transition != events.ThresholdLevelIncreased {
		return
	}
	var nodeID string
	if local.GetInstance() != nil {
		nodeID = local.GetInstance().ID().String()
	}

	transactionID := branchID.TransactionID()
	var transaction *ledgerstate.Transaction
	oldestAttachmentTime := time.Unix(0, 0)
	oldestAttachmentMessageID := tangle.EmptyMessageID
	messagelayer.Tangle().LedgerState.BranchDAG.InclusionState(branchID)
	if !messagelayer.Tangle().Storage.Attachments(transactionID).Consume(func(attachment *tangle.Attachment) {
		messagelayer.Tangle().Storage.Message(attachment.MessageID()).Consume(func(message *tangle.Message) {
			if oldestAttachmentTime.Unix() == 0 || message.IssuingTime().Before(oldestAttachmentTime) {
				oldestAttachmentTime = message.IssuingTime()
				oldestAttachmentMessageID = message.ID()
			}
			if transaction == nil {
				transaction = message.Payload().(*ledgerstate.Transaction)
			}
		})
	}) {
		return
	}

	record := &remotemetrics.BranchConfirmationMetrics{
		Type:               "branchConfirmation",
		NodeID:             nodeID,
		MetricsLevel:       Parameters.MetricsLevel,
		MessageID:          oldestAttachmentMessageID.Base58(),
		BranchID:           branchID.Base58(),
		CreatedTimestamp:   oldestAttachmentTime,
		ConfirmedTimestamp: clock.SyncedTime(),
		DeltaConfirmed:     clock.Since(oldestAttachmentTime).Nanoseconds(),
		InclusionState:     messagelayer.Tangle().LedgerState.BranchDAG.InclusionState(branchID),
	}

	if err := remotelog.RemoteLogger().Send(record); err != nil {
		plugin.Logger().Errorw("Failed to send BranchConfirmationMetrics record", "err", err)
	}
}
