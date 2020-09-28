package drng

import (
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/plugins/drng"
	"github.com/labstack/echo"
)

// randomnessHandler returns the current DRNG randomness used.
func randomnessHandler(c echo.Context) error {
	randomness := drng.Instance().State.Randomness()
	return c.JSON(http.StatusOK, RandomnessResponse{
		Round:      randomness.Round,
		Randomness: randomness.Randomness,
		Timestamp:  randomness.Timestamp,
	})
}

// RandomnessResponse is the HTTP message containing the current DRNG randomness.
type RandomnessResponse struct {
	Round      uint64    `json:"round,omitempty"`
	Timestamp  time.Time `json:"timestamp,omitempty"`
	Randomness []byte    `json:"randomness,omitempty"`
	Error      string    `json:"error,omitempty"`
}
