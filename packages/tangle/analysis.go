package tangle

import "time"

// TableDescription holds the description of the First Approval analysis table.
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

// MsgInfo holds the information of a message.
type MsgInfo struct {
	MsgID                string
	MsgIssuerID          string
	MsgIssuanceTimestamp time.Time
	MsgArrivalTime       time.Time
	MsgSolidTime         time.Time
}

// MsgApproval holds the information of the first approval by issucane, arrival and solid time.
type MsgApproval struct {
	NodeID                  string
	Msg                     MsgInfo
	FirstApproverByIssuance MsgInfo
	FirstApproverByArrival  MsgInfo
	FirstApproverBySolid    MsgInfo
}

// FirstApprovalAnalysis performs the first approval analysis and write the result into a csv.
func (t *Tangle) FirstApprovalAnalysis(nodeID string, filePath string) error {
	return nil
}
