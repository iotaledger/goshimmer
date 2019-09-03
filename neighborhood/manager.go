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

	inboundRequestQueue = 100
	dropQueue           = 100

	Accept = true
	Reject = false

	lifetime = 300 * time.Second
)

type network interface {
	Local() *peer.Local

	sendOutboundRequest(*peer.Peer, *salt.Salt) (bool, error)

	sendInboundReply(*peer.Peer, bool)

	sendDrop(*peer.Peer)
}

type GetKnownPeers func() []*peer.Peer
type PeeringRequest struct {
	Requester *peer.Peer
	Salt      *salt.Salt
}

type manager struct {
	net network
	log *zap.SugaredLogger

	getKnownPeers GetKnownPeers

	inbound  Neighborhood
	outbound Neighborhood

	inboundRequestChan chan PeeringRequest
	inboundReplyChan   chan bool
	inboundDropChan    chan *peer.Peer
	outboundDropChan   chan *peer.Peer

	rejectionFilter Filter

	wg      sync.WaitGroup
	closing chan struct{}
}

func newManager(net network, getKnownPeers GetKnownPeers, log *zap.SugaredLogger) *manager {
	m := &manager{
		net:                net,
		getKnownPeers:      getKnownPeers,
		log:                log,
		closing:            make(chan struct{}),
		rejectionFilter:    make(Filter),
		inboundRequestChan: make(chan PeeringRequest, inboundRequestQueue),
		inboundDropChan:    make(chan *peer.Peer, dropQueue),
		outboundDropChan:   make(chan *peer.Peer, dropQueue),
	}

	m.wg.Add(2)
	go m.loopOutbound()
	go m.loopInbound()

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

	salt := m.net.Local().GetPublicSalt()

Loop:
	for {
		select {
		case <-updateOutbound.C:
			// if there is no updateOutbound, this means doUpdateOutbound is not running
			if updateOutboundDone == nil {
				updateOutboundDone = make(chan struct{})
				// check Public Salt (update outbound distances)
				if salt.Expired() {
					salt = m.updatePublicSalt()
				}

				go m.updateOutbound(updateOutboundDone)
			}
		case <-updateOutboundDone:
			updateOutboundDone = nil
			updateOutbound.Reset(updateOutboundInterval) // updateOutbound again after the given interval
		case peerToDrop := <-m.outboundDropChan:
			if containsPeer(m.outbound.GetPeers(), peerToDrop.ID()) {
				m.outbound.RemovePeer(peerToDrop)
				m.rejectionFilter.AddPeer(peerToDrop)
			}
		case <-m.closing:
			break Loop
		}
	}

	// wait for the updateOutbound to finish
	if updateOutboundDone != nil {
		<-updateOutboundDone
	}
}

func (m *manager) loopInbound() {
	defer m.wg.Done()

	salt := m.net.Local().GetPrivateSalt()

	for {
		select {
		case req := <-m.inboundRequestChan:
			// check Private Salt (update inbound distances)
			if salt.Expired() {
				salt = m.updatePrivateSalt()
			}

			m.updateInbound(req.Requester, req.Salt)
		case peerToDrop := <-m.inboundDropChan:
			if containsPeer(m.inbound.GetPeers(), peerToDrop.ID()) {
				m.inbound.RemovePeer(peerToDrop)
			}
		case <-m.closing:
			return
		}
	}
}

// doUpdateOutbound updates outbound neighbors.
func (m *manager) updateOutbound(done chan<- struct{}) {
	defer func() {
		done <- struct{}{}
	}() // always signal, when the function returns

	// sort verified peers by distance
	distList := peer.SortBySalt(m.net.Local().ID().Bytes(), m.net.Local().GetPublicSalt().GetBytes(), m.getKnownPeers())

	filter := make(Filter)
	filter.AddNeighborhood(m.inbound)  // set filter for inbound neighbors
	filter.AddNeighborhood(m.outbound) // set filter for outbound neighbors

	filteredList := filter.Apply(distList)               // filter out current neighbors
	filteredList = m.rejectionFilter.Apply(filteredList) // filter out previous rejection

	// select new candidate
	candidate := m.inbound.Select(filteredList)
	if candidate.Remote != nil {
		// send peering request
		accepted, _ := m.net.sendOutboundRequest(candidate.Remote, m.net.Local().GetPublicSalt())
		// add candidate to the outbound neighborhood
		if accepted {
			furtherest := m.outbound.Add(candidate)
			// drop furtherest neighbor
			if furtherest != nil {
				m.net.sendDrop(furtherest)
			}
		}
		// add rejecting peer to the rejection filter
		if !accepted {
			m.rejectionFilter.AddPeer(candidate.Remote)
		}
		// TODO: handle err
	}
}

func (m *manager) updateInbound(requester *peer.Peer, salt *salt.Salt) {
	// TODO: check request legitimacy

	reqDistance := peer.NewPeerDistance(m.net.Local().ID().Bytes(), m.net.Local().GetPrivateSalt().GetBytes(), requester)

	candidateList := []peer.PeerDistance{reqDistance}

	// make decision
	toAccept := m.inbound.Select(candidateList)

	// reject request
	if toAccept.Remote == nil {
		m.inboundReplyChan <- Reject
		return
	}

	// accept request
	m.inboundReplyChan <- Accept

	// update inbound neighborhood
	furtherest := m.inbound.Add(toAccept)

	// drop furtherest neighbor
	if furtherest != nil {
		m.net.sendDrop(furtherest)
	}
}

func (m *manager) updatePublicSalt() *salt.Salt {
	salt, _ := salt.NewSalt(lifetime)
	m.net.Local().SetPublicSalt(salt)

	m.outbound.UpdateDistance(m.self().ID().Bytes(), salt.GetBytes())

	return salt
}

func (m *manager) updatePrivateSalt() *salt.Salt {
	salt, _ := salt.NewSalt(lifetime)
	m.net.Local().SetPrivateSalt(salt)

	m.inbound.UpdateDistance(m.self().ID().Bytes(), salt.GetBytes())

	return salt
}

func (m *manager) DropNeighbor(peerToDrop *peer.Peer) {
	m.inboundDropChan <- peerToDrop
	m.outboundDropChan <- peerToDrop
}

// containsPeer returns true if a peer with the given ID is in the list.
func containsPeer(list []*peer.Peer, id peer.ID) bool {
	for _, p := range list {
		if p.ID() == id {
			return true
		}
	}
	return false
}

func (m *manager) AcceptRequest(p *peer.Peer, s *salt.Salt) bool {
	req := PeeringRequest{p, s}
	m.inboundRequestChan <- req
	return <-m.inboundReplyChan
}
