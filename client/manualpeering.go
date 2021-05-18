package client

import (
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/crypto/ed25519"

	"github.com/iotaledger/goshimmer/packages/manualpeering"
	plugin "github.com/iotaledger/goshimmer/plugins/manualpeering"
)

// AddManualPeers adds the provided list of peers to the manual peering layer.
func (api *GoShimmerAPI) AddManualPeers(peers []*peer.Peer) error {
	if err := api.do(http.MethodPost, plugin.RouteManualPeers, peers, nil); err != nil {
		return errors.Wrap(err, "failed to add manual peers via the HTTP API")
	}
	return nil
}

// RemoveManualPeers remove the provided list of peers from the manual peering layer.
func (api *GoShimmerAPI) RemoveManualPeers(keys []ed25519.PublicKey) error {
	peersToRemove := make([]*plugin.PeerToRemove, len(keys))
	for i, key := range keys {
		peersToRemove[i] = &plugin.PeerToRemove{PublicKey: key.String()}
	}
	if err := api.do(http.MethodDelete, plugin.RouteManualPeers, peersToRemove, nil); err != nil {
		return errors.Wrap(err, "failed to remove manual peers via the HTTP API")
	}
	return nil
}

// GetManualKnownPeers gets the list of connected neighbors from the manual peering layer.
func (api *GoShimmerAPI) GetManualKnownPeers(opts ...manualpeering.GetKnownPeersOption) (
	peers []*manualpeering.KnownPeer, err error) {
	conf := manualpeering.BuildGetKnownPeersConfig(opts)
	if err := api.do(http.MethodGet, plugin.RouteManualPeers, conf, &peers); err != nil {
		return nil, errors.Wrap(err, "failed to get manual connected peers from the API")
	}
	return peers, nil
}
