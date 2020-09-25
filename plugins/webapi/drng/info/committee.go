package info

import (
	"encoding/hex"
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/drng"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58"
)

// CommitteeHandler returns the current DRNG committee used.
func CommitteeHandler(c echo.Context) error {
	committee := drng.Instance().State.Committee()
	identities := []string{}
	for _, pk := range committee.Identities {
		identities = append(identities, base58.Encode(pk[:]))
	}
	return c.JSON(http.StatusOK, CommitteeResponse{
		InstanceID:    committee.InstanceID,
		Threshold:     committee.Threshold,
		Identities:    identities,
		DistributedPK: hex.EncodeToString(committee.DistributedPK),
	})
}

// CommitteeResponse is the HTTP message containing the DRNG committee.
type CommitteeResponse struct {
	InstanceID    uint32   `json:"instanceID,omitempty"`
	Threshold     uint8    `json:"threshold,omitempty"`
	Identities    []string `json:"identities,omitempty"`
	DistributedPK string   `json:"distributedPK,omitempty"`
	Error         string   `json:"error,omitempty"`
}
