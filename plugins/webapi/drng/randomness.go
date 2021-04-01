package drng

import (
	"net/http"

	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/plugins/drng"
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
)

// randomnessHandler returns the current DRNG randomness used.
func randomnessHandler(c echo.Context) error {
	randomness := []jsonmodels.Randomness{}
	for _, state := range drng.Instance().State {
		randomness = append(randomness,
			jsonmodels.Randomness{
				InstanceID: state.Committee().InstanceID,
				Round:      state.Randomness().Round,
				Randomness: state.Randomness().Randomness,
				Timestamp:  state.Randomness().Timestamp,
			})
	}

	return c.JSON(http.StatusOK, jsonmodels.RandomnessResponse{
		Randomness: randomness,
	})
}
