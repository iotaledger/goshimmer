package randomness

import (
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/plugins/drng"
	"github.com/labstack/echo"
)

// Handler creates a message of the given payload and
// broadcasts it to the node's neighbors. It returns the message ID if successful.
func Handler(c echo.Context) error {
	randomness := drng.Instance.State.Randomness()
	return c.JSON(http.StatusOK, Response{
		Round:      randomness.Round,
		Randomness: randomness.Randomness,
		Timestamp:  randomness.Timestamp,
	})
}

type Response struct {
	Round      uint64    `json:"round,omitempty"`
	Timestamp  time.Time `json:"timestamp,omitempty"`
	Randomness []byte    `json:"randomness,omitempty"`
	Error      string    `json:"error,omitempty"`
}
