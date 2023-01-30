package client

import (
	"fmt"
	"net/http"

	"github.com/iotaledger/hive.go/core/identity"
	"github.com/pkg/errors"
)

const (
	routeFirewallPeerFaultinessCount = "firewall/peer-faultiness-count"
)

// GetPeerFaultinessCount return number of time peer has been marked as faulty.
func (api *GoShimmerAPI) GetPeerFaultinessCount(peerID identity.ID) (int, error) {
	var count int
	if err := api.do(
		http.MethodGet,
		fmt.Sprintf("%s/%s", routeFirewallPeerFaultinessCount, peerID.EncodeBase58()),
		nil, &count,
	); err != nil {
		return 0, errors.Wrap(err, "failed to fetch peer faultiness details via HTTP API")
	}
	return count, nil
}
