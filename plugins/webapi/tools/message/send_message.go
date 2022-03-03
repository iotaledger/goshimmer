package message

import (
	"fmt"
	"net/http"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

const maxIssuedAwaitTime = 5 * time.Second

func SendMessage(c echo.Context) error {
	if !deps.Tangle.Synced() {
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: tangle.ErrNotSynced.Error()})
	}

	if c.Request().Body == nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: "invalid message, error: request body is missing"})
	}

	var request jsonmodels.SendMessageRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: err.Error()})
	}
	if len(request.Payload) == 0 {
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: "no data provided"})
	}
	if len(request.ParentMessageIDs) == 0 {
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: "no parents provided"})
	}

	var parents tangle.MessageIDsSlice
	references := tangle.NewParentMessageIDs()
	for _, p := range request.ParentMessageIDs {
		for _, ID := range p.MessageIDs {
			msgID, err := tangle.NewMessageID(ID)
			if err != nil {
				return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: fmt.Sprintf("error decoding messageID: %s", ID)})
			}
			references = references.Add(tangle.ParentsType(p.Type), msgID)
			parents = append(parents, msgID)
		}
	}

	// fields set on the node
	issuerPublicKey := deps.Local.PublicKey()
	issuingTime := deps.Tangle.MessageFactory.GetIssuingTime(parents)
	sequenceNumber, err := deps.Tangle.MessageFactory.NextSequenceNumber()
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: err.Error()})
	}
	msgPayload, _, err := payload.GenericDataPayloadFromBytes(request.Payload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: err.Error()})
	}
	nonce, err := deps.Tangle.MessageFactory.DoPOW(references, issuingTime, issuerPublicKey, sequenceNumber, msgPayload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: err.Error()})
	}
	signature, err := deps.Tangle.MessageFactory.Sign(references, issuingTime, issuerPublicKey, sequenceNumber, msgPayload, nonce)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: err.Error()})
	}
	msg, err := tangle.NewMessage(references, issuingTime, issuerPublicKey, sequenceNumber, msgPayload, nonce, signature)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: err.Error()})
	}

	go deps.Tangle.MessageFactory.Events.MessageConstructed.Trigger(msg)

	// await messageProcessed event to be triggered.
	timer := time.NewTimer(maxIssuedAwaitTime)
	defer timer.Stop()
	msgStored := make(chan bool)

	messageStoredClosure := events.NewClosure(func(messageID tangle.MessageID) {
		if messageID.CompareTo(msg.ID()) == 0 {
			msgStored <- true
		}
	})
	deps.Tangle.Storage.Events.MessageStored.Attach(messageStoredClosure)
	defer deps.Tangle.Storage.Events.MessageStored.Detach(messageStoredClosure)
L:
	for {
		select {
		case <-timer.C:
			return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: "message not stored in time"})
		case <-msgStored:
			break L
		}
	}

	return c.JSON(http.StatusOK, jsonmodels.DataResponse{ID: msg.ID().Base58()})
}
