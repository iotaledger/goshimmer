package chat

import (
	"net/http"

	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/chat"
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
)

const (
	maxFromToLength  = 100
	maxMessageLength = 1000
)

func configureWebAPI() {
	deps.Server.POST("chat", SendChatMessage)
}

// SendChatMessage sends a chat message.
func SendChatMessage(c echo.Context) error {
	req := &Request{}
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if len(req.From) > maxFromToLength {
		return c.JSON(http.StatusBadRequest, Response{Error: "sender is too long"})
	}
	if len(req.To) > maxFromToLength {
		return c.JSON(http.StatusBadRequest, Response{Error: "receiver is too long"})
	}
	if len(req.Message) > maxMessageLength {
		return c.JSON(http.StatusBadRequest, Response{Error: "message is too long"})
	}

	chatPayload := chat.NewPayload(req.From, req.To, req.Message)
	msg, err := deps.Tangle.IssuePayload(chatPayload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{MessageID: msg.ID().Base58()})
}

// Request defines the chat message to send.
type Request struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Message string `json:"message"`
}

// Response contains the ID of the message sent.
type Response struct {
	MessageID string `json:"messageID,omitempty"`
	Error     string `json:"error,omitempty"`
}
