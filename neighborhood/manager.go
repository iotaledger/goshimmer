package neighborhood

import (
	"sync"
	"time"

	"github.com/wollac/autopeering/peer"
	"github.com/wollac/autopeering/salt"
	"go.uber.org/zap"
)

const (
	updateOutboundInterval = 10 * time.Second
	repeeringInterval      = 300 * time.Second
)

type network interface {
	Local() *peer.Local

	sendOutboundRequest(*peer.Peer, salt.Salt, func())

	sendInboundReply(*peer.Peer, bool)

	sendDrop(*peer.Peer)
}

type GetKnownPeers func() []*peer.Peer
type manager struct {
	net  network
	boot []*peer.Peer
	log  *zap.SugaredLogger

	getKnownPeers GetKnownPeers

	inbound       Neighborhood
	inboundMutex  sync.RWMutex
	outbound      Neighborhood
	outboundMutex sync.RWMutex
	mutex         sync.Mutex

	wg      sync.WaitGroup
	closing chan struct{}
}

func newManager(net network, getKnownPeers GetKnownPeers, log *zap.SugaredLogger) *manager {
	m := &manager{
		net:           net,
		getKnownPeers: getKnownPeers,
		log:           log,
		closing:       make(chan struct{}),
	}

	m.wg.Add(2)
	go m.loopOutbound()
	go m.loopRepeering()

	return m
}

func (m *manager) self() *peer.Local {
	return m.net.Local()
}

func (m *manager) close() {
	m.log.Debugf("closing")

	close(m.closing)
	m.wg.Wait()
}

func (m *manager) loopOutbound() {
	defer m.wg.Done()

	var (
		updateOutbound     = time.NewTimer(0) // setting this to 0 will cause a trigger right away
		updateOutboundDone chan struct{}
	)
	defer updateOutbound.Stop()

Loop:
	for {
		select {
		case <-updateOutbound.C:
			// if there is no updateOutbound, this means doUpdateOutbound is not running
			if updateOutboundDone == nil {
				updateOutboundDone = make(chan struct{})
				//go m.doUpdateOutbound(updateOutboundDone)
			}
		case <-updateOutboundDone:
			updateOutboundDone = nil
			updateOutbound.Reset(updateOutboundInterval) // updateOutbound again after the given interval
		case <-m.closing:
			break Loop
		}
	}

	// wait for the updateOutbound to finish
	if updateOutboundDone != nil {
		<-updateOutboundDone
	}
}

func (m *manager) loopRepeering() {
	defer m.wg.Done()

	var (
		repeering     = time.NewTimer(0) // setting this to 0 will cause a trigger right away
		repeeringDone chan struct{}
	)
	defer repeering.Stop()

Loop:
	for {
		select {
		case <-repeering.C:
			// if there is no repeering, this means doRepeering is not running
			if repeeringDone == nil {
				repeeringDone = make(chan struct{})
				go m.doRepeering(repeeringDone)
			}
		case <-repeeringDone:
			repeeringDone = nil
			repeering.Reset(repeeringInterval) // repeering again after the given interval
		case <-m.closing:
			break Loop
		}
	}

	// wait for the repeering to finish
	if repeeringDone != nil {
		<-repeeringDone
	}
}

// doReverify pings the oldest known peer.
func (m *manager) doRepeering(done chan<- struct{}) {
	defer func() {
		m.inboundMutex.Unlock()
		m.outboundMutex.Unlock()
		done <- struct{}{}
	}() // always signal, when the function returns

	m.inboundMutex.Lock()
	m.outboundMutex.Lock()

	// update public salt
	newSalt, err := salt.NewSalt(repeeringInterval)
	if err != nil {
		//handle error
	}
	m.net.Local().SetPublicSalt(newSalt)

	// update private salt
	newSalt, err = salt.NewSalt(repeeringInterval)
	if err != nil {
		//handle error
	}
	m.net.Local().SetPrivateSalt(newSalt)

	// uddate distance neighborhood

}

func (m *manager) dropNeighbor(peerToDrop *peer.Peer) {
	checkOutbound := true
	m.inboundMutex.Lock()
	neighbors := []peer.PeerDistance{}
	for _, peer := range m.inbound.Neighbors {
		if peer.Remote == peerToDrop {
			checkOutbound = false
		} else {
			neighbors = append(neighbors, peer)
		}
	}
	m.inbound.Neighbors = neighbors
	m.inboundMutex.Unlock()

	if checkOutbound {
		m.outboundMutex.Lock()
		neighbors := []peer.PeerDistance{}
		for _, peer := range m.outbound.Neighbors {
			if peer.Remote != peerToDrop {
				neighbors = append(neighbors, peer)
			}
		}
		m.outbound.Neighbors = neighbors
		m.outboundMutex.Unlock()
	}
}

// // containsPeer returns true if a peer with the given ID is in the list.
// func containsPeer(list []*mpeer, id peer.ID) bool {
// 	for _, p := range list {
// 		if p.ID() == id {
// 			return true
// 		}
// 	}
// 	return false
// }

// // deletePeer is a helper that deletes the peer with the given index from the list.
// func deletePeer(list []*mpeer, i int) ([]*mpeer, *mpeer) {
// 	p := list[i]

// 	copy(list[i:], list[i+1:])
// 	list[len(list)-1] = nil

// 	return list[:len(list)-1], p
// }

// outbound selection:
// - configuration
// - run
// - neighborhood data
// - ticker to get Verified Peers
// - select next candidate
// - peering request
// - update neighborhood data (potentially drop)
// - update filter data

// inbound selection:
// - configuration
// - run
// - neighborhood data
// - buffered channel to read Peering requests
// - accept/reject
// - update neighborhood data (potentially drop)
// - update filter data
