package firewall

import (
	"sync"

	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/logger"

	"github.com/iotaledger/goshimmer/packages/gossip"
)

// Firewall is a object responsible for taking actions on faulty peers.
type Firewall struct {
	gossipMgr                 *gossip.Manager
	autopeering               *selection.Protocol
	log                       *logger.Logger
	peersFaultinessCountMutex sync.RWMutex
	peersFaultinessCount      map[identity.ID]int
}

// NewFirewall create a new instance of Firewall object.
func NewFirewall(gossipMgr *gossip.Manager, autopeering *selection.Protocol, log *logger.Logger) (*Firewall, error) {
	return &Firewall{
		gossipMgr:            gossipMgr,
		autopeering:          autopeering,
		log:                  log,
		peersFaultinessCount: map[identity.ID]int{},
	}, nil
}

// FaultinessDetails contains information about why the peers is considered faulty.
type FaultinessDetails struct {
	Reason string                 `json:"reason"`
	Info   map[string]interface{} `json:"info"`
}

func (fd *FaultinessDetails) toKVList() []interface{} {
	list := []interface{}{"reason", fd.Reason}
	for k, v := range fd.Info {
		list = append(list, k, v)
	}
	return list
}

// HandleFaultyPeer handles a faulty peer and takes appropriate actions.
func (f *Firewall) HandleFaultyPeer(peerID identity.ID, details *FaultinessDetails) {
	logKVList := append([]interface{}{"peerId", peerID}, details.toKVList()...)
	f.log.Infow("Peer is faulty, executing firewall logic to handle the peer", logKVList...)
	f.incrPeerFaultinessCount(peerID)
	nbr, err := f.gossipMgr.GetNeighbor(peerID)
	if err != nil {
		f.log.Errorw("Can't get neighbor info from the gossip manager", "peerId", peerID, "err", err)
		return
	}
	if nbr.Group == gossip.NeighborsGroupAuto {
		if f.autopeering != nil {
			f.log.Infow(
				"Blocklisting peer in the autopeering selection",
				"peerId", peerID,
			)
			f.autopeering.BlockNeighbor(peerID)
		}
	} else if nbr.Group == gossip.NeighborsGroupManual {
		f.log.Warnw("To the node operator. One of neighbors connected via manual peering acts faulty, no automatic actions taken. Consider removing it from the known peers list.",
			logKVList...)
	}
}

// GetPeerFaultinessCount returns number of times the peer has been considered faulty.
func (f *Firewall) GetPeerFaultinessCount(peerID identity.ID) int {
	f.peersFaultinessCountMutex.RLock()
	defer f.peersFaultinessCountMutex.RUnlock()
	return f.peersFaultinessCount[peerID]
}

func (f *Firewall) incrPeerFaultinessCount(peerID identity.ID) {
	f.peersFaultinessCountMutex.Lock()
	defer f.peersFaultinessCountMutex.Unlock()
	f.peersFaultinessCount[peerID]++
}
