package networkdelay

import (
	"math/rand"
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/plugins/issuer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/labstack/echo"
)

func configureWebAPI() {
	webapi.Server.POST("networkdelay", broadcastNetworkDelayObject)
}

// broadcastNetworkDelayObject creates a message with a network delay object and
// broadcasts it to the node's neighbors. It returns the message ID if successful.
func broadcastNetworkDelayObject(c echo.Context) error {
	// generate random id
	rand.Seed(time.Now().UnixNano())
	var id [32]byte
	if _, err := rand.Read(id[:]); err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Error: err.Error()})
	}

	msg, err := issuer.IssuePayload(NewObject(id, time.Now().UnixNano()))
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, Response{ID: msg.Id().String()})
}

// Response contains the ID of the message sent.
type Response struct {
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}
