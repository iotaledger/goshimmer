package neighborhood

import (
	"math/rand"
	"sync"
	"time"

	"github.com/wollac/autopeering/peer"
	"github.com/wollac/autopeering/salt"
	"go.uber.org/zap"
)

const (
	updateOutboundInterval = 100 * time.Millisecond

	inboundRequestQueue = 100
	dropQueue           = 100

	Accept = true
	Reject = false

	lifetime = 100 * time.Second
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
	m.log.Debugf("closing Done")
}

func (m *Manager) loopOutbound() {
	defer m.wg.Done()

	var (
		updateOutboundDone chan struct{}
		updateOutbound     = time.NewTimer(0) // setting this to 0 will cause a trigger right away
		backoff            = 50
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
				// check salt and update if necessary (this will drop the whole neighborhood)
				if salt.Expired() {
					salt, _ = m.updateSalt()
				}
				//remove potential duplicates
				dup := m.getDuplicates()
				for _, peerToDrop := range dup {
					toDrop := m.inbound.GetPeerFromID(peerToDrop)
					time.Sleep(time.Duration(rand.Intn(backoff)) * time.Millisecond)
					m.outbound.RemovePeer(peerToDrop)
					m.inbound.RemovePeer(peerToDrop)
					if toDrop != nil {
						m.net.DropPeer(toDrop)
					}
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
				m.log.Debug("Outbound Dropped BY ", peerToDrop, " (", len(m.outbound.GetPeers()), ",", len(m.inbound.GetPeers()), ")")
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

Loop:
	for {
		select {
		case req := <-m.inboundRequestChan:
			m.updateInbound(req.Requester, req.Salt)
		case peerToDrop := <-m.inboundDropChan:
			if containsPeer(m.inbound.GetPeers(), peerToDrop) {
				m.inbound.RemovePeer(peerToDrop)
				m.log.Debug("Inbound Dropped BY ", peerToDrop, " (", len(m.outbound.GetPeers()), ",", len(m.inbound.GetPeers()), ")")
			}
		case <-m.inboundGetNeighbors:
			m.inboundNeighbors <- m.inbound.GetPeers()
		case <-m.inboundClosing:
			break Loop
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

	filter := NewFilter()
	filter.AddPeers(m.inbound.GetPeers())  // set filter for inbound neighbors
	filter.AddPeers(m.outbound.GetPeers()) // set filter for outbound neighbors

	filteredList := filter.Apply(distList)               // filter out current neighbors
	filteredList = m.rejectionFilter.Apply(filteredList) // filter out previous rejection

	// select new candidate
	candidate := m.outbound.Select(filteredList)

	if candidate.Remote != nil {
		furtherest, _ := m.outbound.getFurtherest()

		// send peering request
		accepted, _ := m.net.RequestPeering(candidate.Remote, m.net.Local().GetPublicSalt())

		// add candidate to the outbound neighborhood
		if accepted {
			//m.acceptedFilter.AddPeer(candidate.Remote.ID())
			if furtherest.Remote != nil {
				m.outbound.RemovePeer(furtherest.Remote.ID())
				m.net.DropPeer(furtherest.Remote)
				m.log.Debug("Outbound Furtherest removed ", furtherest.Remote.ID())
			}
			m.outbound.Add(candidate)
			m.log.Debug("Peering request TO ", candidate.Remote.ID(), " status ACCEPTED (", len(m.outbound.GetPeers()), ",", len(m.inbound.GetPeers()), ")")
			// drop furtherest neighbor

		}
		// add rejecting peer to the rejection filter
		if !accepted { //&& !ignored {
			m.log.Debug("Peering request TO ", candidate.Remote.ID(), " status REJECTED (", len(m.outbound.GetPeers()), ",", len(m.inbound.GetPeers()), ")")
			m.rejectionFilter.AddPeer(candidate.Remote.ID())
			m.log.Debug("Rejection Filter ", candidate.Remote.ID())
		}
		// TODO: handle err
	}
}

//func (m *Manager) updateInbound(requester *peer.Peer, salt *salt.Salt, done chan<- struct{}) {
func (m *Manager) updateInbound(requester *peer.Peer, salt *salt.Salt) {
	// TODO: check request legitimacy
	//m.log.Debug("Evaluating peering request FROM ", requester.ID())
	reqDistance := peer.NewPeerDistance(m.net.Local().ID().Bytes(), m.net.Local().GetPrivateSalt().GetBytes(), requester)

	candidateList := []peer.PeerDistance{reqDistance}

	filter := NewFilter()
	filter.AddPeers(m.outbound.GetPeers())      // set filter for outbound neighbors
	filteredList := filter.Apply(candidateList) // filter out current neighbors

	// make decision
	toAccept := m.inbound.Select(filteredList)

	// reject request
	if toAccept.Remote == nil {
		m.log.Debug("Peering request FROM ", requester.ID(), " status REJECTED (", len(m.outbound.GetPeers()), ",", len(m.inbound.GetPeers()), ")")
		m.inboundReplyChan <- Reject
		return
	}
	// accept request
	m.inboundReplyChan <- Accept
	furtherest, _ := m.inbound.getFurtherest()
	// drop furtherest neighbor
	if furtherest.Remote != nil {
		m.inbound.RemovePeer(furtherest.Remote.ID())
		m.net.DropPeer(furtherest.Remote)
		m.log.Debug("Inbound Furtherest removed ", furtherest.Remote.ID())
	}
	// update inbound neighborhood
	m.inbound.Add(toAccept)
	m.log.Debug("Peering request FROM ", toAccept.Remote.ID(), " status ACCEPTED (", len(m.outbound.GetPeers()), ",", len(m.inbound.GetPeers()), ")")
}

func (m *Manager) updateSalt() (*salt.Salt, *salt.Salt) {
	pubSalt, _ := salt.NewSalt(lifetime)
	m.net.Local().SetPublicSalt(pubSalt)

	privSalt, _ := salt.NewSalt(lifetime)
	m.net.Local().SetPrivateSalt(privSalt)

	m.rejectionFilter.Clean()

	m.dropNeighborhood(m.inbound)
	m.dropNeighborhood(m.outbound)

	return pubSalt, privSalt
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
	//m.log.Debug("Peering request received from ", p.ID())
	req := PeeringRequest{p, s}
	m.inboundRequestChan <- req
	resp := <-m.inboundReplyChan
	//m.log.Debug("Peering request from ", p.ID(), " evaluated, status ", resp)
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

func (m *Manager) GetInbound() *Neighborhood {
	return m.inbound
}

func (m *Manager) GetOutbound() *Neighborhood {
	return m.outbound
}

func (m *Manager) dropNeighborhood(nh *Neighborhood) {
	for _, peer := range nh.GetPeers() {
		nh.RemovePeer(peer.ID())
		m.net.DropPeer(peer)
	}
}
