package message

import (
	"fmt"
	"net/http"
	"time"

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
	if msg.Nonce() == 0 {
		references := tangle.NewParentMessageIDs()
		msg.ForEachParent(func(parent tangle.Parent) {
			references.Add(parent.Type, parent.ID)
		})
		nonce, err := deps.Tangle.MessageFactory.DoPOW(references, msg.IssuingTime(), msg.IssuerPublicKey(), msg.SequenceNumber(), msg.Payload())
		if err != nil {
			return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: err.Error()})
		}
		// update the nonce
		msg, err = tangle.NewMessage(references, msg.IssuingTime(), msg.IssuerPublicKey(), msg.SequenceNumber(), msg.Payload(), nonce, msg.Signature())
		if err != nil {
			return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: err.Error()})
		}
	}

	deps.Tangle.MessageFactory.Events.MessageConstructed.Trigger(msg)

	// await messageProcessed event to be triggered.
	timer := time.NewTimer(maxIssuedAwaitTime)
	msgProcessed := make(chan bool)
	defer timer.Stop()
L:
	for {
		select {
		case <-timer.C:
			return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: "message not processed in time"})
		case <-msgProcessed:
			break L
		}
	}

	return c.JSON(http.StatusOK, jsonmodels.DataResponse{ID: msg.ID().Base58()})
}
