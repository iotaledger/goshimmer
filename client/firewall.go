package client

import (
	"fmt"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/firewall"
)

const (
	routeFirewallIsPeerFaulty = "firewall/is-peer-faulty"
)

// IsPeerFaulty checks whether node considers the peer faulty.
func (api *GoShimmerAPI) IsPeerFaulty(peerID identity.ID) (*firewall.FaultinessDetails, error) {
	var faultyDetails *firewall.FaultinessDetails
	if err := api.do(
		http.MethodGet,
		fmt.Sprintf("%s/%s", routeFirewallIsPeerFaulty, peerID.EncodeBase58()),
		nil, &faultyDetails,
	); err != nil {
		return nil, errors.Wrap(err, "failed to fetch peer faultiness details via HTTP API")
	}
	return faultyDetails, nil
}
