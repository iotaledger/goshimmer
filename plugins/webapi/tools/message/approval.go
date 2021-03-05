package message

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/identity"
	"github.com/labstack/echo"
)

var fileName = "approval-analysis.csv"

// ApprovalHandler runs the approval analysis.
func ApprovalHandler(c echo.Context) error {
	path := config.Node().String(CfgExportPath)
	res := &ApprovalResponse{}
	res.Err = firstApprovalAnalysis(local.GetInstance().Identity.ID().String(), path+fileName)
	if res.Err != nil {
		c.JSON(http.StatusInternalServerError, res)
	}
	return c.JSON(http.StatusOK, res)
}

// ApprovalResponse is the HTTP response.
type ApprovalResponse struct {
	Err error `json:"error,omitempty"`
}

// region Analysis code implementation /////////////////////////////////////////////////////////////////////////////////

func firstApprovalAnalysis(nodeID string, filePath string) (err error) {
	// If the file doesn't exist, create it, or truncate the file
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)

	// write TableDescription
	if err := w.Write(TableDescription); err != nil {
		return err
	}

	messagelayer.Tangle().Utils.WalkMessageID(func(msgID tangle.MessageID, walker *walker.Walker) {
		approverInfo, err := firstApprovers(msgID)
		// firstApprovers returns an error when the msgID is a tip, thus
		// we want to stop the computation but continue with the future cone iteration.
		if err != nil {
			return
		}

		msgApproval := MsgApproval{
			NodeID:                  nodeID,
			Msg:                     info(msgID),
			FirstApproverByIssuance: approverInfo[byIssuance],
			FirstApproverByArrival:  approverInfo[byArrival],
			FirstApproverBySolid:    approverInfo[bySolid],
		}

		// write msgApproval to file
		if err = w.Write(msgApproval.toCSV()); err != nil {
			return
		}
		w.Flush()
		if err = w.Error(); err != nil {
			return
		}

		messagelayer.Tangle().Storage.Approvers(msgID).Consume(func(approver *tangle.Approver) {
			walker.Push(approver.ApproverMessageID())
		})
	}, tangle.MessageIDs{tangle.EmptyMessageID})

	return
}

// TableDescription holds the description of the First Approval analysis table.
var TableDescription = []string{
	"nodeID",
	"MsgID",
	"MsgIssuerID",
	"MsgIssuanceTime",
	"MsgArrivalTime",
	"MsgSolidTime",
	"ByIssuanceMsgID",
	"ByIssuanceMsgIssuerID",
	"ByIssuanceMsgIssuanceTime",
	"ByIssuanceMsgArrivalTime",
	"ByIssuanceMsgSolidTime",
	"ByArrivalMsgID",
	"ByArrivalMsgIssuerID",
	"ByArrivalMsgIssuanceTime",
	"ByArrivalMsgArrivalTime",
	"ByArrivalMsgSolidTime",
	"BySolidMsgID",
	"BySolidMsgIssuerID",
	"BySolidMsgIssuanceTime",
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

type approverType uint8

const (
	byIssuance approverType = iota
	byArrival
	bySolid
)

// ByIssuance defines a slice of MsgInfo sortable by timestamp issuance.
type ByIssuance []MsgInfo

func (a ByIssuance) Len() int { return len(a) }
func (a ByIssuance) Less(i, j int) bool {
	return a[i].MsgIssuanceTimestamp.Before(a[j].MsgIssuanceTimestamp)
}
func (a ByIssuance) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

// ByArrival defines a slice of MsgInfo sortable by arrival time.
type ByArrival []MsgInfo

func (a ByArrival) Len() int { return len(a) }
func (a ByArrival) Less(i, j int) bool {
	return a[i].MsgArrivalTime.Before(a[j].MsgArrivalTime)
}
func (a ByArrival) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

// BySolid defines a slice of MsgInfo sortable by solid time.
type BySolid []MsgInfo

func (a BySolid) Len() int { return len(a) }
func (a BySolid) Less(i, j int) bool {
	return a[i].MsgSolidTime.Before(a[j].MsgSolidTime)
}
func (a BySolid) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

func firstApprovers(msgID tangle.MessageID) ([]MsgInfo, error) {
	approversInfo := make([]MsgInfo, 0)

	messagelayer.Tangle().Storage.Approvers(msgID).Consume(func(approver *tangle.Approver) {
		approversInfo = append(approversInfo, info(approver.ApproverMessageID()))
	})

	if len(approversInfo) == 0 {
		return nil, fmt.Errorf("message: %v is a tip", msgID)
	}

	result := make([]MsgInfo, 3)

	sort.Sort(ByIssuance(approversInfo))
	result[byIssuance] = approversInfo[0]

	sort.Sort(ByArrival(approversInfo))
	result[byArrival] = approversInfo[0]

	sort.Sort(BySolid(approversInfo))
	result[bySolid] = approversInfo[0]

	return result, nil
}

func info(msgID tangle.MessageID) MsgInfo {
	msgInfo := MsgInfo{
		MsgID: msgID.String(),
	}

	messagelayer.Tangle().Storage.Message(msgID).Consume(func(msg *tangle.Message) {
		msgInfo.MsgIssuanceTimestamp = msg.IssuingTime()
		msgInfo.MsgIssuerID = identity.NewID(msg.IssuerPublicKey()).String()
	})

	messagelayer.Tangle().Storage.MessageMetadata(msgID).Consume(func(msgMetadata *tangle.MessageMetadata) {
		msgInfo.MsgArrivalTime = msgMetadata.ReceivedTime()
		msgInfo.MsgSolidTime = msgMetadata.SolidificationTime()
	}, false)

	return msgInfo
}

func (m MsgApproval) toCSV() (row []string) {
	row = append(row, m.NodeID)
	row = append(row, m.Msg.toCSV()...)
	row = append(row, m.FirstApproverByIssuance.toCSV()...)
	row = append(row, m.FirstApproverByArrival.toCSV()...)
	row = append(row, m.FirstApproverBySolid.toCSV()...)
	return
}

func (m MsgInfo) toCSV() (row []string) {
	row = append(row, []string{
		m.MsgID,
		m.MsgIssuerID,
		fmt.Sprint(m.MsgIssuanceTimestamp.UnixNano()),
		fmt.Sprint(m.MsgArrivalTime.UnixNano()),
		fmt.Sprint(m.MsgSolidTime.UnixNano())}...)

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
