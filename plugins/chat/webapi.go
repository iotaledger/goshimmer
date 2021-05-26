package chat

import (
	"net/http"

	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
)

func configureWebAPI() {
	webapi.Server().POST("chat", sendChatMessage)
}

// broadcastNetworkDelayObject creates a message with a network delay object and
// broadcasts it to the node's neighbors. It returns the message ID if successful.
func sendChatMessage(c echo.Context) error {
	req := &Reuqest{}
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}
	obj := NewPayload(req.From, req.To, req.Message)

	msg, err := messagelayer.Tangle().IssuePayload(obj)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{MessageID: msg.ID().Base58()})
}

// Request defines the chat message to send
type Reuqest struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Message string `json:"message"`
}

// Response contains the ID of the message sent.
type Response struct {
	MessageID string `json:"messageID,omitempty"`
	Error     string `json:"error,omitempty"`
}
