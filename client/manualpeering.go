package client

import (
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/crypto/ed25519"

	"github.com/iotaledger/goshimmer/plugins/manualpeering"
)

const (
	routeManualConnectedPeers = "manualpeering/peers/connected"
	routeManualPeers          = "manualpeering/peers"
)

// AddManualPeers adds the provided list of peers to the manual peering layer.
func (api *GoShimmerAPI) AddManualPeers(peers []*peer.Peer) error {
	if err := api.do(http.MethodPost, routeManualPeers, peers, nil); err != nil {
		return errors.Wrap(err, "failed to add manual peers via the HTTP API")
	}
	return nil
}

// RemoveManualPeers remove the provided list of peers from the manual peering layer.
func (api *GoShimmerAPI) RemoveManualPeers(keys []ed25519.PublicKey) error {
	peersToRemove := make([]*manualpeering.PeerToRemove, len(keys))
	for i, key := range keys {
		peersToRemove[i] = &manualpeering.PeerToRemove{PublicKey: key.String()}
	}
	if err := api.do(http.MethodDelete, routeManualPeers, peersToRemove, nil); err != nil {
		return errors.Wrap(err, "failed to remove manual peers via the HTTP API")
	}
	return nil
}

// GetManualConnectedPeers gets the list of connected neighbors from the manual peering layer.
func (api *GoShimmerAPI) GetManualConnectedPeers() ([]*peer.Peer, error) {
	var peers []*peer.Peer
	if err := api.do(http.MethodGet, routeManualConnectedPeers, nil, &peers); err != nil {
		return nil, errors.Wrap(err, "failed to get manual connected peers from the API")
	}
	return peers, nil
}
