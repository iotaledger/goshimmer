package randomness

import (
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/plugins/drng"
	"github.com/labstack/echo"
)

// Handler returns the current DRNG randomness used.
func Handler(c echo.Context) error {
	randomness := drng.Instance().State.Randomness()
	return c.JSON(http.StatusOK, Response{
		Round:      randomness.Round,
		Randomness: randomness.Randomness,
		Timestamp:  randomness.Timestamp,
	})
}

// Response is the HTTP message containing the current DRNG randomness.
type Response struct {
	Round      uint64    `json:"round,omitempty"`
	Timestamp  time.Time `json:"timestamp,omitempty"`
	Randomness []byte    `json:"randomness,omitempty"`
	Error      string    `json:"error,omitempty"`
}
