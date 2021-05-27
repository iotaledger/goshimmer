package networkdelay

import (
	"math/rand"
	"net/http"
	"time"

	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
)

func configureWebAPI() {
	webapi.Server().POST("networkdelay", broadcastNetworkDelayObject)
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

	now := clock.SyncedTime().UnixNano()

	obj := NewObject(id, now)

	nowWithoutClock := time.Now()

	msg, err := messagelayer.Tangle().IssuePayload(obj)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	sendPoWInfo(obj, time.Since(nowWithoutClock))

	return c.JSON(http.StatusOK, Response{ID: msg.ID().Base58()})
}

// Response contains the ID of the message sent.
type Response struct {
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}
