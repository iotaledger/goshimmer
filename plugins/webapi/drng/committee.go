package drng

import (
	"encoding/hex"
	jsonmodels2 "github.com/iotaledger/goshimmer/packages/jsonmodels"
	"net/http"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/plugins/drng"
)

// committeeHandler returns the current DRNG committee used.
func committeeHandler(c echo.Context) error {
	committees := []jsonmodels2.Committee{}
	for _, state := range drng.Instance().State {
		committees = append(committees, jsonmodels2.Committee{
			InstanceID:    state.Committee().InstanceID,
			Threshold:     state.Committee().Threshold,
			Identities:    identitiesToString(state.Committee().Identities),
			DistributedPK: hex.EncodeToString(state.Committee().DistributedPK),
		})
	}
	return c.JSON(http.StatusOK, jsonmodels2.CommitteeResponse{
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
