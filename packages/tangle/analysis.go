package tangle

import (
	"container/list"
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"
)

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

// FirstApprovalAnalysis performs the first approval analysis and write the result into a csv.
// This function is very heavy to compute especially when starting from the Genesis.
// A better alternative for the future would be to keep this analysis updated as the Tangle grows.
func (t *Tangle) FirstApprovalAnalysis(nodeID string, filePath string) error {
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

	return t.FutureCone(EmptyMessageID, func(msgID MessageID) error {
		approverInfo, err := t.firstApprovers(msgID)
		// firstApprovers returns an error when the msgID is a tip, thus
		// we want to stop the computation but continue with the future cone iteration.
		if err != nil {
			return nil
		}

		msgApproval := MsgApproval{
			NodeID:                  nodeID,
			Msg:                     t.info(msgID),
			FirstApproverByIssuance: approverInfo[byIssuance],
			FirstApproverByArrival:  approverInfo[byArrival],
			FirstApproverBySolid:    approverInfo[bySolid],
		}

		// write msgApproval to file
		if err := w.Write(msgApproval.toCSV()); err != nil {
			return err
		}
		w.Flush()
		if err := w.Error(); err != nil {
			return err
		}
		return nil
	})
}

// FutureCone iterates over the future cone of the given messageID and computes the given function.
func (t *Tangle) FutureCone(messageID MessageID, compute func(ID MessageID) error) error {
	futureConeStack := list.New()
	futureConeStack.PushBack(messageID)

	processedMessages := make(map[MessageID]types.Empty)
	processedMessages[messageID] = types.Void

	for futureConeStack.Len() >= 1 {
		currentStackEntry := futureConeStack.Front()
		currentMessageID := currentStackEntry.Value.(MessageID)
		futureConeStack.Remove(currentStackEntry)

		if err := compute(currentMessageID); err != nil {
			return err
		}

		t.Approvers(currentMessageID).Consume(func(approver *Approver) {
			approverID := approver.ApproverMessageID()
			if _, messageProcessed := processedMessages[approverID]; !messageProcessed {
				futureConeStack.PushBack(approverID)
				processedMessages[approverID] = types.Void
			}
		})
	}

	return nil
}

func (t *Tangle) firstApprovers(msgID MessageID) ([]MsgInfo, error) {
	approversInfo := make([]MsgInfo, 0)

	t.Approvers(msgID).Consume(func(approver *Approver) {
		approversInfo = append(approversInfo, t.info(approver.ApproverMessageID()))
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

func (t *Tangle) info(msgID MessageID) MsgInfo {
	msgInfo := MsgInfo{
		MsgID: msgID.String(),
	}

	t.Message(msgID).Consume(func(msg *Message) {
		msgInfo.MsgIssuanceTimestamp = msg.IssuingTime()
		msgInfo.MsgIssuerID = identity.NewID(msg.IssuerPublicKey()).String()
	})

	t.MessageMetadata(msgID).Consume(func(object objectstorage.StorableObject) {
		msgMetadata := object.(*MessageMetadata)
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
