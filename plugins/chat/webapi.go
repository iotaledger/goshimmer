package chat

import (
	"net/http"

	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
)

const (
	maxFromToLength = 100
	maxBlockLength  = 1000
)

func configureWebAPI() {
	deps.Server.POST("chat", SendChatBlock)
}

// SendChatBlock sends a chat block.
func SendChatBlock(c echo.Context) error {
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
	if len(req.Block) > maxBlockLength {
		return c.JSON(http.StatusBadRequest, Response{Error: "block is too long"})
	}

	// TODO: finish when ratesetter and blockfactory are figured out
	//chatPayload := chat.NewPayload(req.From, req.To, req.Block)
	//blk, err := deps.Protocol.Instance().IssuePayload(chatPayload)
	//if err != nil {
	//	return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	//}

	//return c.JSON(http.StatusOK, Response{BlockID: blk.ID().Base58()})
	return nil
}

// Request defines the chat block to send.
type Request struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Block string `json:"block"`
}

// Response contains the ID of the block sent.
type Response struct {
	BlockID string `json:"blockID,omitempty"`
	Error   string `json:"error,omitempty"`
}
