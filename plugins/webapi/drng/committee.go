package drng

import (
	"encoding/hex"
	"net/http"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
)

// committeeHandler returns the current DRNG committee used.
func committeeHandler(c echo.Context) error {
	committees := []jsonmodels.Committee{}
	for _, state := range deps.DrngInstance.State {
		committees = append(committees, jsonmodels.Committee{
			InstanceID:    state.Committee().InstanceID,
			Threshold:     state.Committee().Threshold,
			Identities:    identitiesToString(state.Committee().Identities),
			DistributedPK: hex.EncodeToString(state.Committee().DistributedPK),
		})
	}
	return c.JSON(http.StatusOK, jsonmodels.CommitteeResponse{
		Committees: committees,
	})
}

func identitiesToString(publicKeys []ed25519.PublicKey) []string {
	identities := []string{}
	for _, pk := range publicKeys {
		identities = append(identities, base58.Encode(pk[:]))
	}
	return identities
}
