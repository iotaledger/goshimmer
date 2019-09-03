package neighborhood

import (
	"sync"
	"time"

	"github.com/wollac/autopeering/peer"
	"github.com/wollac/autopeering/salt"
	"go.uber.org/zap"
)

const (
	updateOutboundInterval = 2 * time.Second

	inboundRequestQueue = 100
	dropQueue           = 100

	Accept = true
	Reject = false

	lifetime = 300 * time.Second
)

type network interface {
	Local() *peer.Local
	RequestPeering(*peer.Peer, *salt.Salt) (bool, error)
	DropPeer(*peer.Peer)
}

type GetKnownPeers func() []*peer.Peer
type PeeringRequest struct {
	Requester *peer.Peer
	Salt      *salt.Salt
}

type Manager struct {
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

func NewManager(net network, getKnownPeers GetKnownPeers, log *zap.SugaredLogger) *Manager {
	m := &Manager{
		net:                net,
		getKnownPeers:      getKnownPeers,
		log:                log,
		closing:            make(chan struct{}),
		rejectionFilter:    make(Filter),
		inboundRequestChan: make(chan PeeringRequest, inboundRequestQueue),
		inboundReplyChan:   make(chan bool),
		inboundDropChan:    make(chan *peer.Peer, dropQueue),
		outboundDropChan:   make(chan *peer.Peer, dropQueue),
		inbound:            Neighborhood{[]peer.PeerDistance{}, 4},
		outbound:           Neighborhood{[]peer.PeerDistance{}, 4},
	}

	m.wg.Add(2)
	go m.loopOutbound()
	go m.loopInbound()

	return m
}

func (m *Manager) self() *peer.Local {
	return m.net.Local()
}

func (m *Manager) Close() {
	m.log.Debugf("closing")

	close(m.closing)
	m.wg.Wait()
}

func (m *Manager) loopOutbound() {
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

func (m *Manager) loopInbound() {
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
func (m *Manager) updateOutbound(done chan<- struct{}) {
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
		m.log.Debug("Sending peering request to ", candidate.Remote.ID())
		accepted, _ := m.net.RequestPeering(candidate.Remote, m.net.Local().GetPublicSalt())

		// add candidate to the outbound neighborhood
		if accepted {
			furtherest := m.outbound.Add(candidate)
			// drop furtherest neighbor
			if furtherest != nil {
				m.net.DropPeer(furtherest)
			}
		}
		// add rejecting peer to the rejection filter
		if !accepted {
			m.rejectionFilter.AddPeer(candidate.Remote)
		}
		// TODO: handle err
	}
}

func (m *Manager) updateInbound(requester *peer.Peer, salt *salt.Salt) {
	// TODO: check request legitimacy
	m.log.Debug("Evaluating peering request from ", requester.ID())
	reqDistance := peer.NewPeerDistance(m.net.Local().ID().Bytes(), m.net.Local().GetPrivateSalt().GetBytes(), requester)

	candidateList := []peer.PeerDistance{reqDistance}

	filter := make(Filter)
	filter.AddNeighborhood(m.outbound) // set filter for outbound neighbors

	filteredList := filter.Apply(candidateList) // filter out current neighbors

	// make decision
	toAccept := m.inbound.Select(filteredList)

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
		m.net.DropPeer(furtherest)
	}
}

func (m *Manager) updatePublicSalt() *salt.Salt {
	salt, _ := salt.NewSalt(lifetime)
	m.net.Local().SetPublicSalt(salt)

	m.outbound.UpdateDistance(m.self().ID().Bytes(), salt.GetBytes())

	return salt
}

func (m *Manager) updatePrivateSalt() *salt.Salt {
	salt, _ := salt.NewSalt(lifetime)
	m.net.Local().SetPrivateSalt(salt)

	m.inbound.UpdateDistance(m.self().ID().Bytes(), salt.GetBytes())

	return salt
}

func (m *Manager) DropNeighbor(peerToDrop *peer.Peer) {
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

func (m *Manager) AcceptRequest(p *peer.Peer, s *salt.Salt) bool {
	m.log.Debug("Peering request received from ", p.ID())
	req := PeeringRequest{p, s}
	m.inboundRequestChan <- req
	resp := <-m.inboundReplyChan
	m.log.Debug("Peering request received from ", p.ID(), " status ", resp)
	return resp
}
