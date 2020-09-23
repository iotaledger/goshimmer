package webapi

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	"github.com/labstack/echo"
)

func init() {
	Server().POST("message/sendPayload", sendPayload)
}

// sendPayload creates a message of the given payload and
// broadcasts it to the node's neighbors. It returns the message ID if successful.
func sendPayload(c echo.Context) error {
	if _, exists := DisabledAPIs[MessageRoot]; exists {
		return c.JSON(http.StatusForbidden, MsgResponse{Error: "Forbidden endpoint"})
	}

	var request MsgRequest
	if err := c.Bind(&request); err != nil {
		Log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, MsgResponse{Error: err.Error()})
	}

	parsedPayload, _, err := tangle.PayloadFromBytes(request.Payload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, MsgResponse{Error: err.Error()})
	}

	msg, err := issuer.IssuePayload(parsedPayload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, MsgResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, MsgResponse{ID: msg.ID().String()})
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
