package drng

import (
	jsonmodels2 "github.com/iotaledger/goshimmer/packages/jsonmodels"
	"net/http"

	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/plugins/drng"
)

// randomnessHandler returns the current DRNG randomness used.
func randomnessHandler(c echo.Context) error {
	randomness := []jsonmodels2.Randomness{}
	for _, state := range drng.Instance().State {
		randomness = append(randomness,
			jsonmodels2.Randomness{
				InstanceID: state.Committee().InstanceID,
				Round:      state.Randomness().Round,
				Randomness: state.Randomness().Randomness,
				Timestamp:  state.Randomness().Timestamp,
			})
	}

	return c.JSON(http.StatusOK, jsonmodels2.RandomnessResponse{
		Randomness: randomness,
	})
}
