package remotelogmetrics

import (
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/remotelogmetrics"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

func onStatementReceived(msg *tangle.Message) {
	if !deps.Tangle.Synced() {
		return
	}

	clockEnabled := !node.IsSkipped(deps.ClockPlugin)
	var myID string
	if deps.Local != nil {
		myID = deps.Local.ID().String()
	}
	deps.Tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *tangle.MessageMetadata) {
		issuedTime := msg.IssuingTime()
		arrivalTime := messageMetadata.ReceivedTime()
		solidTime := messageMetadata.SolidificationTime()
		issuerID := identity.NewID(msg.IssuerPublicKey())
		m := remotelogmetrics.StatementLog{
			NodeID:       myID,
			MsgID:        msg.ID().Base58(),
			IssuerID:     issuerID.String(),
			IssuedTime:   issuedTime,
			ArrivalTime:  arrivalTime,
			SolidTime:    solidTime,
			DeltaArrival: arrivalTime.UnixNano() - issuedTime.UnixNano(),
			DeltaSolid:   solidTime.UnixNano() - issuedTime.UnixNano(),
			Clock:        clockEnabled,
			Sync:         deps.Tangle.Synced(),
			Type:         "statement",
		}
		if err := deps.RemoteLogger.Send(m); err != nil {
			Plugin.Logger().Errorw("Failed to send statement metrics", "err", err)
		}
	})
}
