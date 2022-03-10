package message

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/generics/walker"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/tangle"
)

var fileNameOrphanage = "orphanage-analysis.csv"

// OrphanageHandler runs the orphanage analysis.
func OrphanageHandler(c echo.Context) error {
	// get current executable's path and define output path
	ex, err := os.Executable()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, OrphanageResponse{Err: err.Error()})
	}
	path := filepath.Join(filepath.Dir(ex), fileNameOrphanage)

	// check whether a valid message ID is given in the request
	targetMessageID, err := tangle.NewMessageID(c.QueryParam("msgID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, OrphanageResponse{Err: err.Error()})
	}

	if err = orphanageAnalysis(targetMessageID, path); err != nil {
		return c.JSON(http.StatusInternalServerError, OrphanageResponse{Err: err.Error()})
	}
	return c.JSON(http.StatusOK, OrphanageResponse{})
}

// OrphanageResponse is the HTTP response.
type OrphanageResponse struct {
	Err string `json:"error,omitempty"`
}

// region Analysis code implementation /////////////////////////////////////////////////////////////////////////////////

func orphanageAnalysis(targetMessageID tangle.MessageID, filePath string) error {
	// If the file doesn't exist, create it, or truncate the file
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)

	// write TableDescription
	if err := w.Write(TableDescriptionOrphanage); err != nil {
		return err
	}
	var writeErr error
	deps.Tangle.Utils.WalkMessageID(func(msgID tangle.MessageID, walker *walker.Walker[tangle.MessageID]) {
		approverMessageIDs := deps.Tangle.Utils.ApprovingMessageIDs(msgID)
		if len(approverMessageIDs) == 0 {
			deps.Tangle.Storage.Message(msgID).Consume(func(message *tangle.Message) {
				deps.Tangle.Storage.MessageMetadata(msgID).Consume(func(messageMetadata *tangle.MessageMetadata) {
					msgApproval := MsgInfoOrphanage{
						MsgID:                msgID,
						MsgIssuerID:          message.IssuerPublicKey(),
						MsgIssuanceTimestamp: message.IssuingTime(),
						MsgArrivalTime:       messageMetadata.ReceivedTime(),
						MsgSolidTime:         messageMetadata.SolidificationTime(),
						MsgApprovedBy:        deps.Tangle.Utils.MessageApprovedBy(targetMessageID, msgID),
					}

					// write msgApproval to file
					if writeErr = w.Write(msgApproval.toCSV()); writeErr != nil {
						return
					}
					w.Flush()
					if writeErr = w.Error(); writeErr != nil {
						return
					}
				})
			})
			return
		}

		// continue walking
		for approverMessageID := range approverMessageIDs {
			walker.Push(approverMessageID)
		}
	}, tangle.NewMessageIDs(targetMessageID))

	return writeErr
}

// TableDescriptionOrphanage holds the description of the First Approval analysis table.
var TableDescriptionOrphanage = []string{
	"MsgID",
	"MsgIssuerID",
	"MsgIssuanceTime",
	"MsgArrivalTime",
	"MsgSolidTime",
	"MsgApprovedBy",
}

// MsgInfoOrphanage holds the information of a message.
type MsgInfoOrphanage struct {
	MsgID                tangle.MessageID
	MsgIssuerID          ed25519.PublicKey
	MsgIssuanceTimestamp time.Time
	MsgArrivalTime       time.Time
	MsgSolidTime         time.Time
	MsgApprovedBy        bool
}

func (m MsgInfoOrphanage) toCSV() (row []string) {
	row = append(row, []string{
		m.MsgID.Base58(),
		m.MsgIssuerID.String(),
		m.MsgIssuanceTimestamp.String(),
		m.MsgArrivalTime.String(),
		m.MsgSolidTime.String(),
		fmt.Sprint(m.MsgApprovedBy),
	}...)

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
