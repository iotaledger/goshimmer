package neighborhood

import (
	"sync"
	"time"

	"github.com/wollac/autopeering/peer"
	"github.com/wollac/autopeering/salt"
	"go.uber.org/zap"
)

const (
	updateOutboundInterval = 1 * time.Millisecond

	inboundRequestQueue = 100
	dropQueue           = 100

	Accept = true
	Reject = false

	lifetime = 300 * time.Second
)

type Network interface {
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
	net Network
	log *zap.SugaredLogger

	getKnownPeers GetKnownPeers

	inbound  *Neighborhood
	outbound *Neighborhood

	inboundRequestChan chan PeeringRequest
	inboundReplyChan   chan bool
	inboundDropChan    chan peer.ID
	outboundDropChan   chan peer.ID

	inboundGetNeighbors  chan struct{}
	inboundNeighbors     chan []*peer.Peer
	outboundGetNeighbors chan struct{}
	outboundNeighbors    chan []*peer.Peer

	rejectionFilter *Filter

	wg              sync.WaitGroup
	inboundClosing  chan struct{}
	outboundClosing chan struct{}
}

func NewManager(net Network, getKnownPeers GetKnownPeers, log *zap.SugaredLogger) *Manager {
	m := &Manager{
		net:                  net,
		getKnownPeers:        getKnownPeers,
		log:                  log,
		inboundClosing:       make(chan struct{}),
		outboundClosing:      make(chan struct{}),
		rejectionFilter:      NewFilter(),
		inboundRequestChan:   make(chan PeeringRequest, inboundRequestQueue),
		inboundReplyChan:     make(chan bool),
		inboundDropChan:      make(chan peer.ID, dropQueue),
		inboundGetNeighbors:  make(chan struct{}),
		inboundNeighbors:     make(chan []*peer.Peer),
		outboundGetNeighbors: make(chan struct{}),
		outboundNeighbors:    make(chan []*peer.Peer),
		outboundDropChan:     make(chan peer.ID, dropQueue),
		inbound: &Neighborhood{
			Neighbors: []peer.PeerDistance{},
			Size:      4},
		outbound: &Neighborhood{
			Neighbors: []peer.PeerDistance{},
			Size:      4},
	}
	return m
}

func (m *Manager) Run() {
	m.wg.Add(2)
	go m.loopOutbound()
	go m.loopInbound()
}

func (m *Manager) self() *peer.Local {
	return m.net.Local()
}

func (m *Manager) Close() {
	m.log.Debugf("closing")

	m.inboundClosing <- struct{}{}
	m.outboundClosing <- struct{}{}
	m.wg.Wait()
	// close(m.inboundClosing)
	// close(m.outboundClosing)
	m.log.Debugf("closing Done")
}

func (m *Manager) loopOutbound() {
	defer m.wg.Done()

	var (
		updateOutboundDone chan struct{}
		updateOutbound     = time.NewTimer(0) // setting this to 0 will cause a trigger right away
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
				// remove potential duplicates
				dup := m.getDuplicates()
				for _, peerToDrop := range dup {
					m.outbound.RemovePeer(peerToDrop)
					//m.net.DropPeer(peerToDrop)
				}
				go m.updateOutbound(updateOutboundDone)
			}
		case <-updateOutboundDone:
			updateOutboundDone = nil
			updateOutbound.Reset(updateOutboundInterval) // updateOutbound again after the given interval
		case peerToDrop := <-m.outboundDropChan:
			if containsPeer(m.outbound.GetPeers(), peerToDrop) {
				m.outbound.RemovePeer(peerToDrop)
				m.rejectionFilter.AddPeer(peerToDrop)
			}
		case <-m.outboundGetNeighbors:
			m.outboundNeighbors <- m.outbound.GetPeers()
		case <-m.outboundClosing:
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

	var (
		updateInboundDone chan struct{}
		salt              = m.net.Local().GetPrivateSalt()
	)

Loop:
	for {
		select {
		case req := <-m.inboundRequestChan:
			if updateInboundDone == nil {
				updateInboundDone = make(chan struct{})
				// check Private Salt (update inbound distances)
				if salt.Expired() {
					salt = m.updatePrivateSalt()
				}
				go m.updateInbound(req.Requester, req.Salt, updateInboundDone)
			}
		case <-updateInboundDone:
			updateInboundDone = nil
		case peerToDrop := <-m.inboundDropChan:
			if containsPeer(m.inbound.GetPeers(), peerToDrop) {
				m.inbound.RemovePeer(peerToDrop)
			}
		case <-m.inboundGetNeighbors:
			m.inboundNeighbors <- m.inbound.GetPeers()
		case <-m.inboundClosing:
			break Loop
		}
	}
	// wait for the updateOutbound to finish
	if updateInboundDone != nil {
		<-updateInboundDone
	}
}

// doUpdateOutbound updates outbound neighbors.
func (m *Manager) updateOutbound(done chan<- struct{}) {
	defer func() {
		done <- struct{}{}
	}() // always signal, when the function returns

	// sort verified peers by distance
	distList := peer.SortBySalt(m.net.Local().ID().Bytes(), m.net.Local().GetPublicSalt().GetBytes(), m.getKnownPeers())

	filter := NewFilter()
	filter.AddPeers(m.inbound.GetPeers())  // set filter for inbound neighbors
	filter.AddPeers(m.outbound.GetPeers()) // set filter for outbound neighbors

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
			m.rejectionFilter.AddPeer(candidate.Remote.ID())
		}
		// TODO: handle err
	}
}

func (m *Manager) updateInbound(requester *peer.Peer, salt *salt.Salt, done chan<- struct{}) {
	defer func() {
		done <- struct{}{}
	}() // always signal, when the function returns

	// TODO: check request legitimacy
	m.log.Debug("Evaluating peering request from ", requester.ID())
	reqDistance := peer.NewPeerDistance(m.net.Local().ID().Bytes(), m.net.Local().GetPrivateSalt().GetBytes(), requester)

	candidateList := []peer.PeerDistance{reqDistance}

	filter := NewFilter()
	filter.AddPeers(m.outbound.GetPeers())      // set filter for outbound neighbors
	filteredList := filter.Apply(candidateList) // filter out current neighbors

	// make decision
	toAccept := m.inbound.Select(filteredList)

	// reject request
	if toAccept.Remote == nil {
		m.inboundReplyChan <- Reject
		return
	}

	// update inbound neighborhood
	furtherest := m.inbound.Add(toAccept)

	// drop furtherest neighbor
	if furtherest != nil {
		m.net.DropPeer(furtherest)
	}
	// accept request
	m.inboundReplyChan <- Accept
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

func (m *Manager) DropNeighbor(peerToDrop peer.ID) {
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

func (m *Manager) GetNeighbors() []*peer.Peer {
	neighbors := []*peer.Peer{}
	m.inboundGetNeighbors <- struct{}{}
	m.outboundGetNeighbors <- struct{}{}
	neighbors = append(neighbors, <-m.inboundNeighbors...)
	neighbors = append(neighbors, <-m.outboundNeighbors...)
	return neighbors
}

func (m *Manager) getDuplicates() []peer.ID {
	d := []peer.ID{}
	for _, peer := range m.inbound.GetPeers() {
		if containsPeer(m.outbound.GetPeers(), peer.ID()) {
			d = append(d, peer.ID())
		}
	}
	return d
}
