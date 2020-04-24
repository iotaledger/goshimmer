package committee

import (
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/drng"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/labstack/echo"
)

// Handler returns the current DRNG committee used.
func Handler(c echo.Context) error {
	committee := drng.Instance().State.Committee()
	return c.JSON(http.StatusOK, Response{
		InstanceID:    committee.InstanceID,
		Threshold:     committee.Threshold,
		Identities:    committee.Identities,
		DistributedPK: committee.DistributedPK,
	})
}

// Response is the HTTP message containing the DRNG committee.
type Response struct {
	InstanceID    uint32              `json:"instanceID,omitempty"`
	Threshold     uint8               `json:"threshold,omitempty"`
	Identities    []ed25519.PublicKey `json:"identitites,omitempty"`
	DistributedPK []byte              `json:"distributedPK,omitempty"`
	Error         string              `json:"error,omitempty"`
}
