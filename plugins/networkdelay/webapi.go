package networkdelay

import (
	"math/rand"
	"net/http"
	"time"

	"github.com/labstack/echo"
)

func configureWebAPI() {
	deps.Server.POST("networkdelay", broadcastNetworkDelayPayload)
}

// broadcastNetworkDelayPayload creates a block with a network delay object and
// broadcasts it to the node's neighbors. It returns the block ID if successful.
func broadcastNetworkDelayPayload(c echo.Context) error {
	// generate random id
	rand.Seed(time.Now().UnixNano())
	var id [32]byte
	if _, err := rand.Read(id[:]); err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Error: err.Error()})
	}

	//now := time.Now().UnixNano()
	//
	//payload := NewPayload(id, now)
	//
	//nowWithoutClock := time.Now()

	// TODO: implement when issuing blocks is figured out
	//blk, err := deps.Tangle.CreateBlock(payload)
	//if err != nil {
	//	return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	//}

	// sendPoWInfo(payload, time.Since(nowWithoutClock))

	// return c.JSON(http.StatusOK, Response{ID: blk.ID().Base58()})
	return nil
}

// Response contains the ID of the block sent.
type Response struct {
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}
