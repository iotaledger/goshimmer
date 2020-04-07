package committee

import (
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/drng"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/labstack/echo"
)

// Handler creates a message of the given payload and
// broadcasts it to the node's neighbors. It returns the message ID if successful.
func Handler(c echo.Context) error {
	committee := drng.Instance.State.Committee()
	return c.JSON(http.StatusOK, Response{
		InstanceID:    committee.InstanceID,
		Threshold:     committee.Threshold,
		Identities:    committee.Identities,
		DistributedPK: committee.DistributedPK,
	})
}

type Response struct {
	InstanceID    uint32              `json:"instanceID,omitempty"`
	Threshold     uint8               `json:"threshold,omitempty"`
	Identities    []ed25519.PublicKey `json:"identitites,omitempty"`
	DistributedPK []byte              `json:"distributedPK,omitempty"`
	Error         string              `json:"error,omitempty"`
}
