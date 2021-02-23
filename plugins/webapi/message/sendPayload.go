package message

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/labstack/echo"
)

// sendPayloadHandler creates a message of the given payload and
// broadcasts it to the node's neighbors. It returns the message ID if successful.
func sendPayloadHandler(c echo.Context) error {
	var request SendPayloadRequest
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, SendPayloadResponse{Error: err.Error()})
	}

	parsedPayload, _, err := payload.FromBytes(request.Payload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, SendPayloadResponse{Error: err.Error()})
	}

	msg, err := issuer.IssuePayload(parsedPayload, messagelayer.Tangle())
	if err != nil {
		return c.JSON(http.StatusBadRequest, SendPayloadResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, SendPayloadResponse{ID: msg.ID().String()})
}

// SendPayloadResponse contains the ID of the message sent.
type SendPayloadResponse struct {
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

// SendPayloadRequest contains the message to send.
type SendPayloadRequest struct {
	Payload []byte `json:"payload"`
}
