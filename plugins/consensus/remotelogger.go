package consensus

import (
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	clockplugin "github.com/iotaledger/goshimmer/plugins/clock"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
	"github.com/iotaledger/goshimmer/plugins/syncbeaconfollower"
	"github.com/iotaledger/hive.go/node"
)

const (
	remoteLogType = "statement"
)

var (
	remoteLogger *remotelog.RemoteLoggerConn
	myID         string
	clockEnabled bool
)

func configureRemoteLogger() {
	remoteLogger = remotelog.RemoteLogger()

	if local.GetInstance() != nil {
		myID = local.GetInstance().ID().String()
	}

	clockEnabled = !node.IsSkipped(clockplugin.Plugin())
}

func sendToRemoteLog(msgID, issuerID string, issuedTime, arrivalTime, solidTime int64) {
	m := statementLog{
		NodeID:       myID,
		MsgID:        msgID,
		IssuerID:     issuerID,
		IssuedTime:   issuedTime,
		ArrivalTime:  arrivalTime,
		SolidTime:    solidTime,
		DeltaArrival: arrivalTime - issuedTime,
		DeltaSolid:   solidTime - issuedTime,
		Clock:        clockEnabled,
		Sync:         syncbeaconfollower.Synced(),
		Type:         remoteLogType,
	}
	_ = remoteLogger.Send(m)
}

type statementLog struct {
	NodeID       string `json:"nodeID"`
	MsgID        string `json:"msgID"`
	IssuerID     string `json:"issuerID"`
	IssuedTime   int64  `json:"issuedTime"`
	ArrivalTime  int64  `json:"arrivalTime"`
	SolidTime    int64  `json:"solidTime"`
	DeltaArrival int64  `json:"deltaArrival"`
	DeltaSolid   int64  `json:"deltaSolid"`
	Clock        bool   `json:"clock"`
	Sync         bool   `json:"sync"`
	Type         string `json:"type"`
}
