package client

import (
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/crypto/ed25519"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/manualpeering"
)

const (
	routeManualPeers = "manualpeering/peers"
)

// AddManualPeers adds the provided list of peers to the manual peering layer.
func (api *GoShimmerAPI) AddManualPeers(peers []*manualpeering.KnownPeerToAdd) error {
	if err := api.do(http.MethodPost, routeManualPeers, peers, nil); err != nil {
		return errors.Wrap(err, "failed to add manual peers via the HTTP API")
	}
	return nil
}

// RemoveManualPeers remove the provided list of peers from the manual peering layer.
func (api *GoShimmerAPI) RemoveManualPeers(keys []ed25519.PublicKey) error {
	peersToRemove := make([]*jsonmodels.PeerToRemove, len(keys))
	for i, key := range keys {
		peersToRemove[i] = &jsonmodels.PeerToRemove{PublicKey: key}
	}
	if err := api.do(http.MethodDelete, routeManualPeers, peersToRemove, nil); err != nil {
		return errors.Wrap(err, "failed to remove manual peers via the HTTP API")
	}
	return nil
}

// GetManualPeers gets the list of connected neighbors from the manual peering layer.
func (api *GoShimmerAPI) GetManualPeers(opts ...manualpeering.GetPeersOption) (
	peers []*manualpeering.KnownPeer, err error) {
	conf := manualpeering.BuildGetPeersConfig(opts)
	if err := api.do(http.MethodGet, routeManualPeers, conf, &peers); err != nil {
		return nil, errors.Wrap(err, "failed to get manual connected peers from the API")
	}
	return peers, nil
}
