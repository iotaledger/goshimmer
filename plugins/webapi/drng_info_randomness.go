package webapi

import (
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/plugins/drng"
	"github.com/labstack/echo"
)

func init() {
	Server().GET("drng/info/randomness", randomnessHandler)
}

// randomnessHandler returns the current DRNG randomness used.
func randomnessHandler(c echo.Context) error {
	if _, exists := DisabledAPIs[DrngRoot]; exists {
		return c.JSON(http.StatusForbidden, RandomnessResponse{Error: "Forbidden endpoint"})
	}

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
