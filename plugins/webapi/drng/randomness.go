package drng

import (
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/plugins/drng"
	"github.com/labstack/echo"
)

// randomnessHandler returns the current DRNG randomness used.
func randomnessHandler(c echo.Context) error {
	randomness := []Randomness{}
	for _, state := range drng.Instance().State {
		randomness = append(randomness,
			Randomness{
				InstanceID: state.Committee().InstanceID,
				Round:      state.Randomness().Round,
				Randomness: state.Randomness().Randomness,
				Timestamp:  state.Randomness().Timestamp,
			})
	}

	return c.JSON(http.StatusOK, RandomnessResponse{
		Randomness: randomness,
	})
}

// RandomnessResponse is the HTTP message containing the current DRNG randomness.
type RandomnessResponse struct {
	Randomness []Randomness `json:"randomness,omitempty"`
	Error      string       `json:"error,omitempty"`
}

// Randomness defines the content of new randomness.
type Randomness struct {
	InstanceID uint32    `json:"instanceID,omitempty"`
	Round      uint64    `json:"round,omitempty"`
	Timestamp  time.Time `json:"timestamp,omitempty"`
	Randomness []byte    `json:"randomness,omitempty"`
}
