package firewall

import (
	"sync"

	"github.com/iotaledger/hive.go/core/autopeering/selection"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/node/p2p"
)

// Firewall is a object responsible for taking actions on faulty peers.
type Firewall struct {
	p2pManager                *p2p.Manager
	autopeering               *selection.Protocol
	log                       *logger.Logger
	peersFaultinessCountMutex sync.RWMutex
	peersFaultinessCount      map[identity.ID]int
}

// NewFirewall create a new instance of Firewall object.
func NewFirewall(p2pManager *p2p.Manager, autopeering *selection.Protocol, log *logger.Logger) (*Firewall, error) {
	return &Firewall{
		p2pManager:           p2pManager,
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
	nbr, err := f.p2pManager.GetNeighbor(peerID)
	if err != nil {
		f.log.Errorw("Can't get neighbor info from the gossip manager", "peerId", peerID, "err", err)
		return
	}
	if nbr.Group == p2p.NeighborsGroupAuto {
		if f.autopeering != nil {
			f.log.Infow(
				"Blocklisting peer in the autopeering selection",
				"peerId", peerID,
			)
			f.autopeering.BlockNeighbor(peerID)
		}
	} else if nbr.Group == p2p.NeighborsGroupManual {
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
