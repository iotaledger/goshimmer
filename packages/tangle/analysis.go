package tangle

import "time"

var TableDescription = []string{
	"nodeID",
	"MsgID",
	"MsgIssuerID",
	"MsgArrivalTime",
	"MsgSolidTime",
	"ByIssuanceMsgID",
	"ByIssuanceMsgIssuerID",
	"ByIssuanceMsgArrivalTime",
	"ByIssuanceMsgSolidTime",
	"ByArrivalMsgID",
	"ByArrivalMsgIssuerID",
	"ByArrivalMsgArrivalTime",
	"ByArrivalMsgSolidTime",
	"BySolidMsgID",
	"BySolidMsgIssuerID",
	"BySolidMsgArrivalTime",
	"BySolidMsgSolidTime",
}

var filePath string

type MsgInfo struct {
	MsgID                string
	MsgIssuerID          string
	MsgIssuanceTimestamp time.Time
	MsgArrivalTime       time.Time
	MsgSolidTime         time.Time
}

type MsgApproval struct {
	NodeID                  string
	Msg                     MsgInfo
	FirstApproverByIssuance MsgInfo
	FirstApproverByArrival  MsgInfo
	FirstApproverBySolid    MsgInfo
}

func (t *Tangle) ApprovalAnalysis(nodeID string, filePath string) error {
	return nil
}
