package remotelogmetrics

import (
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/remotelogmetrics"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/clock"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
)

func onStatementReceived(msg *tangle.Message) {
	if !messagelayer.Tangle().Synced() {
		return
	}

	clockEnabled := !node.IsSkipped(clock.Plugin())
	var myID string
	if local.GetInstance() != nil {
		myID = local.GetInstance().ID().String()
	}
	messagelayer.Tangle().Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *tangle.MessageMetadata) {
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
			Sync:         messagelayer.Tangle().Synced(),
			Type:         "statement",
		}
		if err := remotelog.RemoteLogger().Send(m); err != nil {
			plugin.Logger().Errorw("Failed to send statement metrics", "err", err)
		}
	})
}
