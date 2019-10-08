package main

import (
	"math/rand"
	"time"

	"github.com/wollac/autopeering/neighborhood"
	"github.com/wollac/autopeering/peer"
	"github.com/wollac/autopeering/salt"
	"github.com/wollac/autopeering/simulation/visualizer"
)

type simNet struct {
	loc  *peer.Local
	self *peer.Peer
	mgr  map[peer.ID]*neighborhood.Manager
	rand *rand.Rand
}

func (n simNet) Local() *peer.Local {
	return n.loc
}

func (n simNet) DropPeer(p *peer.Peer) {
	//time.Sleep(time.Duration(n.rand.Intn(max-min+1)+min) * time.Microsecond)
	status.Append(idMap[p.ID()], idMap[n.self.ID()], DROPPED)
	n.mgr[p.ID()].DropNeighbor(n.self.ID())
	timestamp := time.Since(StartTime)
	linkChan <- Event{DROPPED, idMap[p.ID()], idMap[n.self.ID()], timestamp}
	if vEnabled {
		visualizer.RemoveLink(p.ID().String(), n.self.ID().String())
		//visualizer.RemoveLink(n.self.ID().String(), p.ID().String())
	}
}

func (n simNet) RequestPeering(p *peer.Peer, s *salt.Salt) (bool, error) {
	//time.Sleep(time.Duration(n.rand.Intn(max-min+1)+min) * time.Microsecond)
	var response bool

	from := idMap[n.self.ID()]
	to := idMap[p.ID()]
	status.Append(from, to, OUTBOUND)
	status.Append(to, from, INCOMING)

	select {
	case <-time.After(1 * time.Second): // timeout
		response = false
	default:
		response = n.mgr[p.ID()].AcceptRequest(n.self, s)
	}

	if response {
		status.Append(from, to, ACCEPTED)
		timestamp := time.Since(StartTime)
		linkChan <- Event{ESTABLISHED, from, to, timestamp}
		if vEnabled {
			visualizer.AddLink(n.self.ID().String(), p.ID().String())
		}
	} else {
		status.Append(from, to, REJECTED)
	}
	return response, nil
}

func (n simNet) GetKnownPeers() []*peer.Peer {
	var list []*peer.Peer
	list = append(list, allPeers[:idMap[n.self.ID()]]...)
	list = append(list, allPeers[idMap[n.self.ID()]+1:]...)
	return list
}
