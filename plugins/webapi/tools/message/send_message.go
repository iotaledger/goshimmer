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

// SendMessage is the handler for tools/message endpoint.
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

	references := tangle.NewParentMessageIDs()
	for _, p := range request.ParentMessageIDs {
		for _, ID := range p.MessageIDs {
			msgID, err := tangle.NewMessageID(ID)
			if err != nil {
				return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: fmt.Sprintf("error decoding messageID: %s", ID)})
			}
			references = references.Add(tangle.ParentsType(p.Type), msgID)
		}
	}
	msgPayload, _, err := payload.GenericDataPayloadFromBytes(request.Payload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: err.Error()})
	}
	msg, err := deps.Tangle.MessageFactory.IssuePayloadWithReferences(msgPayload, references)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: err.Error()})
	}

	// await MessageScheduled event to be triggered.
	timer := time.NewTimer(maxIssuedAwaitTime)
	defer timer.Stop()
	msgScheduled := make(chan bool)

	messageScheduledClosure := events.NewClosure(func(messageID tangle.MessageID) {
		if messageID.CompareTo(msg.ID()) == 0 {
			msgScheduled <- true
		}
	})
	deps.Tangle.Scheduler.Events.MessageScheduled.Attach(messageScheduledClosure)
	defer deps.Tangle.Scheduler.Events.MessageScheduled.Detach(messageScheduledClosure)

	go deps.Tangle.MessageFactory.Events.MessageConstructed.Trigger(msg)
L:
	for {
		select {
		case <-timer.C:
			return c.JSON(http.StatusBadRequest, jsonmodels.DataResponse{Error: "message not scheduled in time"})
		case <-msgScheduled:
			break L
		}
	}

	return c.JSON(http.StatusOK, jsonmodels.DataResponse{ID: msg.ID().Base58()})
}
