package message

import (
	"fmt"
	"net/http"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	"github.com/labstack/echo"
)

// sendPayload creates a message of the given payload and
// broadcasts it to the node's neighbors. It returns the message ID if successful.
func sendPayload(c echo.Context) error {
	var request MsgRequest
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, MsgResponse{Error: err.Error()})
	}

	parsedPayload, _, err := payload.FromBytes(request.Payload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, MsgResponse{Error: "not a valid payload"})
	}

	if len(parsedPayload.Bytes()) > payload.MaxDataPayloadSize {
		msg := fmt.Sprintf("maximum payload size of %d bytes exceeded", payload.MaxDataPayloadSize)
		log.Info(msg)
		return c.JSON(http.StatusBadRequest, Response{Error: msg})
	}


	msg, err := issuer.IssuePayload(parsedPayload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, MsgResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, MsgResponse{ID: msg.Id().String()})
}

// MsgResponse contains the ID of the message sent.
type MsgResponse struct {
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

// MsgRequest contains the message to send.
type MsgRequest struct {
	Payload []byte `json:"payload"`
}
