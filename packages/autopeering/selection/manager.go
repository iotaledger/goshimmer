package selection

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer/service"
	"github.com/iotaledger/goshimmer/packages/autopeering/salt"
	"github.com/iotaledger/hive.go/logger"
)

const (
	accept = true
	reject = false

	// buffer size of the channels handling inbound requests and drops.
	queueSize = 10
)

// A network represents the communication layer for the manager.
type network interface {
	local() *peer.Local

	RequestPeering(*peer.Peer, *salt.Salt) (bool, error)
	SendPeeringDrop(*peer.Peer)
}

type peeringRequest struct {
	peer *peer.Peer
	salt *salt.Salt
}

type manager struct {
	net               network
	getPeersToConnect func() []*peer.Peer
	log               *logger.Logger
	dropOnUpdate      bool          // set true to drop all neighbors when the salt is updated
	requiredServices  []service.Key // services required in order to select/be selected during autopeering

	inbound  *Neighborhood
	outbound *Neighborhood

	rejectionFilter *Filter

	dropChan    chan peer.ID
	requestChan chan peeringRequest
	replyChan   chan bool

	wg      sync.WaitGroup
	closing chan struct{}
}

func newManager(net network, peersFunc func() []*peer.Peer, log *logger.Logger, config Config) *manager {
	return &manager{
		net:               net,
		getPeersToConnect: peersFunc,
		log:               log,
		dropOnUpdate:      config.DropOnUpdate,
		requiredServices:  config.RequiredServices,
		inbound:           NewNeighborhood(inboundNeighborSize),
		outbound:          NewNeighborhood(outboundNeighborSize),
		rejectionFilter:   NewFilter(),
		dropChan:          make(chan peer.ID, queueSize),
		requestChan:       make(chan peeringRequest, queueSize),
		replyChan:         make(chan bool, 1),
		closing:           make(chan struct{}),
	}
}

func (m *manager) start() {
	if m.getPublicSalt() == nil || m.getPrivateSalt() == nil {
		m.updateSalt()
	}
	m.wg.Add(1)
	go m.loop()
}

func (m *manager) close() {
	close(m.closing)
	m.wg.Wait()
}

func (m *manager) getID() peer.ID {
	return m.net.local().ID()
}

func (m *manager) getPublicSalt() *salt.Salt {
	return m.net.local().GetPublicSalt()
}

func (m *manager) getPrivateSalt() *salt.Salt {
	return m.net.local().GetPrivateSalt()
}

func (m *manager) getNeighbors() []*peer.Peer {
	var neighbors []*peer.Peer
	neighbors = append(neighbors, m.inbound.GetPeers()...)
	neighbors = append(neighbors, m.outbound.GetPeers()...)

	return neighbors
}

func (m *manager) getInNeighbors() []*peer.Peer {
	return m.inbound.GetPeers()
}

func (m *manager) getOutNeighbors() []*peer.Peer {
	return m.outbound.GetPeers()
}

func (m *manager) requestPeering(p *peer.Peer, s *salt.Salt) bool {
	var status bool
	select {
	case m.requestChan <- peeringRequest{p, s}:
		status = <-m.replyChan
	default:
		// a full queue should count as a failed request
		status = false
	}
	return status
}

func (m *manager) removeNeighbor(id peer.ID) {
	m.dropChan <- id
}

func (m *manager) loop() {
	defer m.wg.Done()

	var updateOutResultChan chan peer.PeerDistance
	updateTimer := time.NewTimer(0) // setting this to 0 will cause a trigger right away
	defer updateTimer.Stop()

Loop:
	for {
		select {

		// update the outbound neighbors
		case <-updateTimer.C:
			updateOutResultChan = make(chan peer.PeerDistance)
			// check salt and update if necessary
			if m.getPublicSalt().Expired() {
				m.updateSalt()
			}
			// check for new peers to connect to in a separate go routine
			go m.updateOutbound(updateOutResultChan)

		// handle the result of updateOutbound
		case req := <-updateOutResultChan:
			if req.Remote != nil {
				// if the peer is already in inbound, do not add it and remove it from inbound
				if p := m.inbound.RemovePeer(req.Remote.ID()); p != nil {
					m.triggerPeeringEvent(true, req.Remote, false)
					m.dropPeering(p)
				} else {
					m.addNeighbor(m.outbound, req)
					m.triggerPeeringEvent(true, req.Remote, true)
				}
			}
			// call updateOutbound again after the given interval
			updateOutResultChan = nil
			updateTimer.Reset(m.getUpdateTimeout())

		// handle a drop request
		case id := <-m.dropChan:
			droppedPeer := m.inbound.RemovePeer(id)
			if p := m.outbound.RemovePeer(id); p != nil {
				droppedPeer = p
				m.rejectionFilter.AddPeer(id)
				// if not yet updating, trigger an immediate update
				if updateOutResultChan == nil && updateTimer.Stop() {
					updateTimer.Reset(0)
				}
			}
			if droppedPeer != nil {
				m.dropPeering(droppedPeer)
			}

		// handle an inbound request
		case req := <-m.requestChan:
			status := m.handleInRequest(req)
			// trigger in the main loop to guarantee order of events
			m.triggerPeeringEvent(false, req.peer, status)

		// on close, exit the loop
		case <-m.closing:
			break Loop
		}
	}

	// wait for the updateOutbound to finish
	if updateOutResultChan != nil {
		<-updateOutResultChan
	}
}

func (m *manager) getUpdateTimeout() time.Duration {
	result := outboundUpdateInterval
	if m.outbound.IsFull() {
		result = fullOutboundUpdateInterval
	}
	saltExpiration := time.Until(m.getPublicSalt().GetExpiration())
	if saltExpiration < result {
		result = saltExpiration
	}
	return result
}

// updateOutbound updates outbound neighbors.
func (m *manager) updateOutbound(resultChan chan<- peer.PeerDistance) {
	var result peer.PeerDistance
	defer func() { resultChan <- result }() // assure that a result is always sent to the channel

	// sort verified peers by distance
	distList := peer.SortBySalt(m.getID().Bytes(), m.getPublicSalt().GetBytes(), m.getPeersToConnect())

	// filter out current neighbors
	filter := m.getConnectedFilter()
	filteredList := filter.Apply(distList)
	// filter out previous rejections
	filteredList = m.rejectionFilter.Apply(filteredList)
	if len(filteredList) == 0 {
		return
	}

	// select new candidate
	candidate := m.outbound.Select(filteredList)
	if candidate.Remote == nil {
		return
	}

	// send peering request
	status, err := m.net.RequestPeering(candidate.Remote, m.getPublicSalt())
	if err != nil {
		m.rejectionFilter.AddPeer(candidate.Remote.ID())
		m.log.Debugw("error requesting peering",
			"id", candidate.Remote.ID(),
			"addr", candidate.Remote.Address(), "err", err,
		)
		return
	}
	if !status {
		m.rejectionFilter.AddPeer(candidate.Remote.ID())
		m.triggerPeeringEvent(true, candidate.Remote, false)
		return
	}

	result = candidate
}

func (m *manager) handleInRequest(req peeringRequest) (resp bool) {
	resp = reject
	defer func() { m.replyChan <- resp }() // assure that a response is always issued

	if !m.isValidNeighbor(req.peer) {
		return
	}

	reqDistance := peer.NewPeerDistance(m.getID().Bytes(), m.getPrivateSalt().GetBytes(), req.peer)
	filter := m.getConnectedFilter()
	filteredList := filter.Apply([]peer.PeerDistance{reqDistance})
	if len(filteredList) == 0 {
		return
	}

	toAccept := m.inbound.Select(filteredList)
	if toAccept.Remote == nil {
		return
	}

	m.addNeighbor(m.inbound, toAccept)
	resp = accept
	return
}

func (m *manager) addNeighbor(nh *Neighborhood, toAdd peer.PeerDistance) {
	// drop furthest neighbor if necessary
	if furthest, _ := nh.getFurthest(); furthest.Remote != nil {
		if p := nh.RemovePeer(furthest.Remote.ID()); p != nil {
			m.dropPeering(p)
		}
	}
	nh.Add(toAdd)
}

func (m *manager) updateSalt() {
	public, _ := salt.NewSalt(saltLifetime)
	m.net.local().SetPublicSalt(public)
	private, _ := salt.NewSalt(saltLifetime)
	m.net.local().SetPrivateSalt(private)

	// clean the rejection filter
	m.rejectionFilter.Clean()

	if !m.dropOnUpdate { // update distance without dropping neighbors
		m.outbound.UpdateDistance(m.getID().Bytes(), m.getPublicSalt().GetBytes())
		m.inbound.UpdateDistance(m.getID().Bytes(), m.getPrivateSalt().GetBytes())
	} else { // drop all the neighbors
		m.dropNeighborhood(m.inbound)
		m.dropNeighborhood(m.outbound)
	}

	m.log.Debugw("salt updated",
		"public", saltLifetime,
		"private", saltLifetime,
	)
	Events.SaltUpdated.Trigger(&SaltUpdatedEvent{Self: m.getID(), Public: public, Private: private})
}

func (m *manager) dropNeighborhood(nh *Neighborhood) {
	for _, p := range nh.GetPeers() {
		nh.RemovePeer(p.ID())
		m.dropPeering(p)
	}
}

// dropPeering sends the peering drop over the network and triggers the corresponding event.
func (m *manager) dropPeering(p *peer.Peer) {
	m.net.SendPeeringDrop(p)

	m.log.Debugw("peering dropped",
		"id", p.ID(),
		"#out", m.outbound,
		"#in", m.inbound,
	)
	Events.Dropped.Trigger(&DroppedEvent{Self: m.getID(), DroppedID: p.ID()})
}

func (m *manager) getConnectedFilter() *Filter {
	filter := NewFilter()
	filter.AddPeer(m.getID())              //set filter for oneself
	filter.AddPeers(m.inbound.GetPeers())  // set filter for inbound neighbors
	filter.AddPeers(m.outbound.GetPeers()) // set filter for outbound neighbors
	return filter
}

// isValidNeighbor returns whether the given peer is a valid neighbor candidate.
func (m *manager) isValidNeighbor(p *peer.Peer) bool {
	// do not connect to oneself
	if m.getID() == p.ID() {
		return false
	}
	// reject if required services are missing
	for _, reqService := range m.requiredServices {
		if p.Services().Get(reqService) == nil {
			return false
		}
	}
	return true
}

func (m *manager) triggerPeeringEvent(isOut bool, p *peer.Peer, status bool) {
	if isOut {
		m.log.Debugw("peering requested",
			"direction", "out",
			"status", status,
			"to", p.ID(),
			"#out", m.outbound,
			"#in", m.inbound,
		)
		Events.OutgoingPeering.Trigger(&PeeringEvent{Self: m.getID(), Peer: p, Status: status})
	} else {
		m.log.Debugw("peering requested",
			"direction", "in",
			"status", status,
			"from", p.ID(),
			"#out", m.outbound,
			"#in", m.inbound,
		)
		Events.IncomingPeering.Trigger(&PeeringEvent{Self: m.getID(), Peer: p, Status: status})
	}
}
