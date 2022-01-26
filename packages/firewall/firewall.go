package firewall

import (
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/logger"

	"github.com/iotaledger/goshimmer/packages/gossip"
)

// Firewall is a object responsible for taking actions on faulty peers.
type Firewall struct {
	gossipMgr   *gossip.Manager
	autopeering *selection.Protocol
	log         *logger.Logger
}

// NewFirewall create a new instance of Firewall object.
func NewFirewall(gossipMgr *gossip.Manager, autopeering *selection.Protocol, log *logger.Logger) *Firewall {
	return &Firewall{
		gossipMgr:   gossipMgr,
		autopeering: autopeering,
		log:         log,
	}
}

// FaultinessDetails contains information about why the peers is considered faulty.
type FaultinessDetails struct {
	Reason string
	Info   map[string]interface{}
}

func (fd *FaultinessDetails) toKVList() []interface{} {
	list := []interface{}{"reason", fd.Reason}
	for k, v := range fd.Info {
		list = append(list, k, v)
	}
	return list
}

// OnFaultyPeer handles a faulty peer and takes appropriate actions.
func (f *Firewall) OnFaultyPeer(p *peer.Peer, details *FaultinessDetails) {
	f.log.Info("Peer is faulty, executing firewall logic to handle the peer",
		"peerId", p.ID(), details.toKVList())
	nbr, err := f.gossipMgr.GetNeighbor(p.ID())
	if err != nil {
		f.log.Errorw("Can't get neighbor info from the gossip manager", "peerId", p.ID())
		return
	}
	if nbr.Group == gossip.NeighborsGroupAuto {
		if f.autopeering != nil {
			f.log.Infow(
				"Blocklisting peer in the autopeering selection",
				"peerId", p.ID(),
			)
			f.autopeering.BlockNeighbor(p.ID())
		}
	} else if nbr.Group == gossip.NeighborsGroupManual {
		f.log.Warnw("To the node operator. One of neighbors connected via manual peering acts faulty, no automatic actions taken. Consider removing it from the known peers list.",
			"neighborId", p.ID(), details.toKVList())
	}
}
