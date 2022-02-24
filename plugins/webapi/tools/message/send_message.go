package message

import (
	"fmt"
	"net/http"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

const maxIssuedAwaitTime = 5 * time.Second

func SendMessage(c echo.Context) error {
	if !deps.Tangle.Synced() {
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: tangle.ErrNotSynced.Error()})
	}
	deps.Tangle.Booker.Lock()
	defer deps.Tangle.Booker.Unlock()

	if c.Request().Body == nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: "invalid message, error: request body is missing"})
	}

	var request jsonmodels.DataRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: err.Error()})
	}
	if len(request.Data) == 0 {
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: "no data provided"})
	}

	msg, _, err := tangle.MessageFromBytes(request.Data)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: fmt.Sprintf("error: %s", err.Error())})
	}

	if msg.Version() != tangle.MessageVersion {
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: "invalid message version"})
	}
	references := tangle.NewParentMessageIDs()
	msg.ForEachParent(func(parent tangle.Parent) {
		references = references.Add(parent.Type, parent.ID)
	})

	// fields set on the node
	issuerPublicKey := deps.Local.PublicKey()
	var parents tangle.MessageIDsSlice
	msg.ForEachParent(func(parent tangle.Parent) {
		parents = append(parents, parent.ID)
	})
	issuingTime := deps.Tangle.MessageFactory.GetIssuingTime(parents)
	sequenceNumber, err := deps.Tangle.MessageFactory.NextSequenceNumber()
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: err.Error()})
	}
	nonce, err := deps.Tangle.MessageFactory.DoPOW(references, issuingTime, issuerPublicKey, sequenceNumber, msg.Payload())
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: err.Error()})
	}
	signature, err := deps.Tangle.MessageFactory.Sign(references, issuingTime, issuerPublicKey, sequenceNumber, msg.Payload(), nonce)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: err.Error()})
	}
	msg, err = tangle.NewMessage(references, issuingTime, issuerPublicKey, sequenceNumber, msg.Payload(), nonce, signature)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: err.Error()})
	}

	go deps.Tangle.MessageFactory.Events.MessageConstructed.Trigger(msg)

	// await messageProcessed event to be triggered.
	timer := time.NewTimer(maxIssuedAwaitTime)
	msgStored := make(chan bool)
	defer timer.Stop()

	deps.Tangle.Storage.Events.MessageStored.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		if messageID.CompareTo(msg.ID()) == 0 {
			msgStored <- true
		}
	}))
L:
	for {
		select {
		case <-timer.C:
			return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: "message not processed in time"})
		case <-msgStored:
			break L
		}
	}

	return c.JSON(http.StatusOK, jsonmodels.DataResponse{ID: msg.ID().Base58()})
}
