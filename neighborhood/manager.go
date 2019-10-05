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

	inboundRequestQueue = 1000
	dropQueue           = 1000

	accept = true
	reject = false
)

// A Network represents the communication layer for the Manager.
type Network interface {
	Local() *peer.Local

	RequestPeering(*peer.Peer, *salt.Salt) (bool, error)
	DropPeer(*peer.Peer)
}

type peeringRequest struct {
	peer *peer.Peer
	salt *salt.Salt
}

type Manager struct {
	net       Network
	lifetime  time.Duration
	peersFunc func() []*peer.Peer

	log *zap.SugaredLogger

	inbound  *Neighborhood
	outbound *Neighborhood

	inboundRequestChan chan peeringRequest
	inboundReplyChan   chan bool
	inboundDropChan    chan peer.ID
	outboundDropChan   chan peer.ID

	rejectionFilter *Filter

	wg              sync.WaitGroup
	inboundClosing  chan struct{}
	outboundClosing chan struct{}
}

func NewManager(net Network, lifetime time.Duration, peersFunc func() []*peer.Peer, log *zap.SugaredLogger) *Manager {
	m := &Manager{
		net:                net,
		lifetime:           lifetime,
		peersFunc:          peersFunc,
		log:                log,
		inboundClosing:     make(chan struct{}),
		outboundClosing:    make(chan struct{}),
		rejectionFilter:    NewFilter(),
		inboundRequestChan: make(chan peeringRequest, inboundRequestQueue),
		inboundReplyChan:   make(chan bool),
		inboundDropChan:    make(chan peer.ID, dropQueue),
		outboundDropChan:   make(chan peer.ID, dropQueue),
		inbound: &Neighborhood{
			Neighbors: []peer.PeerDistance{},
			Size:      4},
		outbound: &Neighborhood{
			Neighbors: []peer.PeerDistance{},
			Size:      4},
	}
	return m
}

func (m *Manager) Start() {
	// create valid salts
	m.updateSalt()

	m.wg.Add(2)
	go m.loopOutbound()
	go m.loopInbound()
}

func (m *Manager) Close() {
	close(m.inboundClosing)
	close(m.outboundClosing)
	m.wg.Wait()
}

func (m *Manager) loopOutbound() {
	defer m.wg.Done()

	var (
		updateOutboundDone chan struct{}
		updateOutbound     = time.NewTimer(0) // setting this to 0 will cause a trigger right away
		backoff            = 10
	)
	defer updateOutbound.Stop()

Loop:
	for {
		select {
		case <-updateOutbound.C:
			// if there is no updateOutbound, this means doUpdateOutbound is not running
			if updateOutboundDone == nil {
				updateOutboundDone = make(chan struct{})

				// check salt and update if necessary (this will drop the whole neighborhood)
				if m.net.Local().GetPublicSalt().Expired() {
					m.updateSalt()
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

		// on close, exit the loop
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
			m.updateInbound(req.peer, req.salt)
		case peerToDrop := <-m.inboundDropChan:
			if containsPeer(m.inbound.GetPeers(), peerToDrop) {
				m.inbound.RemovePeer(peerToDrop)
				m.log.Debug("Inbound Dropped BY ", peerToDrop, " (", len(m.outbound.GetPeers()), ",", len(m.inbound.GetPeers()), ")")
			}

		// on close, exit the loop
		case <-m.inboundClosing:
			break Loop
		}
	}
}

// updateOutbound updates outbound neighbors.
func (m *Manager) updateOutbound(done chan<- struct{}) {
	defer func() {
		done <- struct{}{}
	}() // always signal, when the function returns

	// sort verified peers by distance
	distList := peer.SortBySalt(m.net.Local().ID().Bytes(), m.net.Local().GetPublicSalt().GetBytes(), m.peersFunc())

	filter := NewFilter()
	filter.AddPeers(m.inbound.GetPeers())  // set filter for inbound neighbors
	filter.AddPeers(m.outbound.GetPeers()) // set filter for outbound neighbors

	filteredList := filter.Apply(distList)               // filter out current neighbors
	filteredList = m.rejectionFilter.Apply(filteredList) // filter out previous rejection

	// select new candidate
	candidate := m.outbound.Select(filteredList)

	if candidate.Remote != nil {
		furthest, _ := m.outbound.getFurthest()

		// send peering request
		mySalt := m.net.Local().GetPublicSalt()
		accepted, _ := m.net.RequestPeering(candidate.Remote, mySalt)

		// add candidate to the outbound neighborhood
		if accepted {
			//m.acceptedFilter.AddPeer(candidate.Remote.ID())
			if furthest.Remote != nil {
				m.outbound.RemovePeer(furthest.Remote.ID())
				m.net.DropPeer(furthest.Remote)
				m.log.Debug("Outbound furthest removed ", furthest.Remote.ID())
			}
			m.outbound.Add(candidate)
			m.log.Debug("Peering request TO ", candidate.Remote.ID(), " status ACCEPTED (", len(m.outbound.GetPeers()), ",", len(m.inbound.GetPeers()), ")")
			// drop furthest neighbor

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
		m.inboundReplyChan <- reject
		return
	}
	// accept request
	m.inboundReplyChan <- accept
	furthest, _ := m.inbound.getFurthest()
	// drop furthest neighbor
	if furthest.Remote != nil {
		m.inbound.RemovePeer(furthest.Remote.ID())
		m.net.DropPeer(furthest.Remote)
		m.log.Debug("Inbound furthest removed ", furthest.Remote.ID())
	}
	// update inbound neighborhood
	m.inbound.Add(toAccept)
	m.log.Debug("Peering request FROM ", toAccept.Remote.ID(), " status ACCEPTED (", len(m.outbound.GetPeers()), ",", len(m.inbound.GetPeers()), ")")
}

func (m *Manager) updateSalt() (*salt.Salt, *salt.Salt) {
	pubSalt, _ := salt.NewSalt(m.lifetime)
	m.net.Local().SetPublicSalt(pubSalt)

	privSalt, _ := salt.NewSalt(m.lifetime)
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
	m.inboundRequestChan <- peeringRequest{p, s}
	return <-m.inboundReplyChan
}

func (m *Manager) GetNeighbors() []*peer.Peer {
	var neighbors []*peer.Peer

	neighbors = append(neighbors, m.inbound.GetPeers()...)
	neighbors = append(neighbors, m.outbound.GetPeers()...)

	return neighbors
}

func (m *Manager) getDuplicates() []peer.ID {
	var d []peer.ID

	for _, p := range m.inbound.GetPeers() {
		if containsPeer(m.outbound.GetPeers(), p.ID()) {
			d = append(d, p.ID())
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
	for _, p := range nh.GetPeers() {
		nh.RemovePeer(p.ID())
		m.net.DropPeer(p)
	}
}
