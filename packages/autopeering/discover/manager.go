package discover

import (
	"math/rand"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	"github.com/iotaledger/goshimmer/packages/autopeering/server"
	"github.com/iotaledger/hive.go/logger"
)

const (
	// PingExpiration is the time until a peer verification expires.
	PingExpiration = 12 * time.Hour
	// MaxPeersInResponse is the maximum number of peers returned in DiscoveryResponse.
	MaxPeersInResponse = 6
	// MaxServices is the maximum number of services a peer can support.
	MaxServices = 5

	// VersionNum specifies the expected version number for this Protocol.
	VersionNum = 0
)

type network interface {
	local() *peer.Local

	ping(*peer.Peer) error
	discoveryRequest(*peer.Peer) ([]*peer.Peer, error)
}

type manager struct {
	mutex        sync.Mutex // protects active and replacement
	active       []*mpeer
	replacements []*mpeer

	net network
	log *logger.Logger

	wg      sync.WaitGroup
	closing chan struct{}
}

func newManager(net network, masters []*peer.Peer, log *logger.Logger) *manager {
	m := &manager{
		active:       make([]*mpeer, 0, maxManaged),
		replacements: make([]*mpeer, 0, maxReplacements),
		net:          net,
		log:          log,
		closing:      make(chan struct{}),
	}
	m.loadInitialPeers(masters)

	return m
}

func (m *manager) start() {
	m.wg.Add(1)
	go m.loop()
}

func (m *manager) self() peer.ID {
	return m.net.local().ID()
}

func (m *manager) close() {
	close(m.closing)
	m.wg.Wait()
}

func (m *manager) loop() {
	defer m.wg.Done()

	var (
		reverify     = time.NewTimer(0) // setting this to 0 will cause a trigger right away
		reverifyDone chan struct{}

		query     = time.NewTimer(server.ResponseTimeout) // trigger the first query after the reverify
		queryNext chan time.Duration
	)
	defer reverify.Stop()
	defer query.Stop()

Loop:
	for {
		select {
		// start verification, if not yet running
		case <-reverify.C:
			// if there is no reverifyDone, this means doReverify is not running
			if reverifyDone == nil {
				reverifyDone = make(chan struct{})
				go m.doReverify(reverifyDone)
			}

		// reset verification
		case <-reverifyDone:
			reverifyDone = nil
			reverify.Reset(reverifyInterval) // reverify again after the given interval

		// start requesting new peers, if no yet running
		case <-query.C:
			if queryNext == nil {
				queryNext = make(chan time.Duration)
				go m.doQuery(queryNext)
			}

		// on query done, reset time to given duration
		case d := <-queryNext:
			queryNext = nil
			query.Reset(d)

		// on close, exit the loop
		case <-m.closing:
			break Loop
		}
	}

	// wait for spawned goroutines to finish
	if reverifyDone != nil {
		<-reverifyDone
	}
	if queryNext != nil {
		<-queryNext
	}
}

// doReverify pings the oldest active peer.
func (m *manager) doReverify(done chan<- struct{}) {
	defer close(done)

	p := m.peerToReverify()
	if p == nil {
		return // nothing can be reverified
	}
	m.log.Debugw("reverifying",
		"id", p.ID(),
		"addr", p.Address(),
	)

	var err error
	for i := 0; i < reverifyTries; i++ {
		err = m.net.ping(unwrapPeer(p))
		if err == nil {
			break
		} else {
			m.log.Debugw("ping failed",
				"id", p.ID(),
				"addr", p.Address(),
				"err", err,
			)
			time.Sleep(1 * time.Second)
		}
	}

	// could not verify the peer
	if err != nil {
		m.mutex.Lock()
		defer m.mutex.Unlock()

		m.active, _ = deletePeerByID(m.active, p.ID())
		m.log.Debugw("remove dead",
			"peer", p,
		)
		Events.PeerDeleted.Trigger(&DeletedEvent{Peer: unwrapPeer(p)})

		// add a random replacement, if available
		if len(m.replacements) > 0 {
			var r *mpeer
			m.replacements, r = deletePeer(m.replacements, rand.Intn(len(m.replacements)))
			m.active = pushPeer(m.active, r, maxManaged)
		}
		return
	}

	// no need to do anything here, as the peer is bumped when handling the pong
}

// peerToReverify returns the oldest peer, or nil if empty.
func (m *manager) peerToReverify() *mpeer {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if len(m.active) == 0 {
		return nil
	}
	// the last peer is the oldest
	return m.active[len(m.active)-1]
}

// updatePeer moves the peer with the given ID to the front of the list of managed peers.
// It returns 0 if there was no peer with that id, otherwise the verifiedCount of the updated peer is returned.
func (m *manager) updatePeer(update *peer.Peer) uint {
	id := update.ID()
	for i, p := range m.active {
		if p.ID() == id {
			if i > 0 {
				//  move i-th peer to the front
				copy(m.active[1:], m.active[:i])
			}
			// replace first mpeer with a wrap of the updated peer
			m.active[0] = &mpeer{
				Peer:          *update,
				verifiedCount: p.verifiedCount + 1,
				lastNewPeers:  p.lastNewPeers,
			}
			return p.verifiedCount + 1
		}
	}
	return 0
}

func (m *manager) addReplacement(p *mpeer) bool {
	if containsPeer(m.replacements, p.ID()) {
		return false // already in the list
	}
	m.replacements = unshiftPeer(m.replacements, p, maxReplacements)
	return true
}

func (m *manager) loadInitialPeers(masters []*peer.Peer) {
	var peers []*peer.Peer

	db := m.net.local().Database()
	if db != nil {
		peers = db.SeedPeers()
	}
	peers = append(peers, masters...)
	for _, p := range peers {
		m.addDiscoveredPeer(p)
	}
}

// addDiscoveredPeer adds a newly discovered peer that has never been verified or pinged yet.
// It returns true, if the given peer was new and added, false otherwise.
func (m *manager) addDiscoveredPeer(p *peer.Peer) bool {
	// never add the local peer
	if p.ID() == m.self() {
		return false
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if containsPeer(m.active, p.ID()) {
		return false
	}
	m.log.Debugw("discovered",
		"peer", p,
	)

	mp := wrapPeer(p)
	if len(m.active) >= maxManaged {
		return m.addReplacement(mp)
	}

	m.active = pushPeer(m.active, mp, maxManaged)
	return true
}

// addVerifiedPeer adds a new peer that has just been successfully pinged.
// It returns true, if the given peer was new and added, false otherwise.
func (m *manager) addVerifiedPeer(p *peer.Peer) bool {
	// never add the local peer
	if p.ID() == m.self() {
		return false
	}

	m.log.Debugw("verified",
		"peer", p,
		"services", p.Services(),
	)

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// if already in the list, move it to the front
	if v := m.updatePeer(p); v > 0 {
		// trigger the event only for the first time the peer is updated
		if v == 1 {
			Events.PeerDiscovered.Trigger(&DiscoveredEvent{Peer: p})
		}
		return false
	}

	mp := wrapPeer(p)
	mp.verifiedCount = 1

	if len(m.active) >= maxManaged {
		return m.addReplacement(mp)
	}
	// trigger the event only when the peer is added to active
	Events.PeerDiscovered.Trigger(&DiscoveredEvent{Peer: p})

	// new nodes are added to the front
	m.active = unshiftPeer(m.active, mp, maxManaged)
	return true
}

// getRandomPeers returns a list of randomly selected peers.
func (m *manager) getRandomPeers(n int, minVerified uint) []*peer.Peer {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if n > len(m.active) {
		n = len(m.active)
	}

	peers := make([]*peer.Peer, 0, n)
	for _, i := range rand.Perm(len(m.active)) {
		if len(peers) == n {
			break
		}

		mp := m.active[i]
		if mp.verifiedCount < minVerified {
			continue
		}
		peers = append(peers, unwrapPeer(mp))
	}

	return peers
}

// getVerifiedPeers returns all the currently managed peers that have been verified at least once.
func (m *manager) getVerifiedPeers() []*mpeer {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	peers := make([]*mpeer, 0, len(m.active))
	for _, mp := range m.active {
		if mp.verifiedCount == 0 {
			continue
		}
		peers = append(peers, mp)
	}

	return peers
}

// isKnown returns true if the manager is keeping track of that peer.
func (m *manager) isKnown(id peer.ID) bool {
	if id == m.self() {
		return true
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	return containsPeer(m.active, id) || containsPeer(m.replacements, id)
}
