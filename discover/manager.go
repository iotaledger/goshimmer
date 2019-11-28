package discover

import (
	"math/rand"
	"sync"
	"time"

	"github.com/iotaledger/autopeering-sim/peer"
	"go.uber.org/zap"
)

const (
	reverifyInterval = 10 * time.Second
	reverifyTries    = 2

	maxKnow         = 1000
	maxReplacements = 10
)

type network interface {
	local() *peer.Local

	ping(*peer.Peer) error
	discoveryRequest(*peer.Peer) ([]*peer.Peer, error)
}

type manager struct {
	mutex        sync.Mutex // protects  known and replacement
	known        []*mpeer
	replacements []*mpeer

	net network
	log *zap.SugaredLogger

	wg      sync.WaitGroup
	closing chan struct{}
}

func newManager(net network, masters []*peer.Peer, log *zap.SugaredLogger) *manager {
	m := &manager{
		known:        make([]*mpeer, 0, maxKnow),
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

		query     = time.NewTimer(0)
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

// doReverify pings the oldest known peer.
func (m *manager) doReverify(done chan<- struct{}) {
	defer close(done)

	p := m.peerToReverify()
	if p == nil {
		return // nothing can be reverified
	}

	m.log.Debug("doReverify")

	var err error
	for i := 0; i < reverifyTries; i++ {
		err = m.net.ping(unwrapPeer(p))
		if err == nil {
			break
		}
	}

	// could not verify the peer
	if err != nil {
		m.mutex.Lock()
		defer m.mutex.Unlock()

		m.known, _ = deletePeerByID(m.known, p.ID())
		m.log.Debugw("remove dead",
			"peer", p,
			"err", err,
		)
		Events.PeerDeleted.Trigger(&DeletedEvent{Peer: unwrapPeer(p)})

		// add a random replacement, if available
		if len(m.replacements) > 0 {
			var r *mpeer
			m.replacements, r = deletePeer(m.replacements, rand.Intn(len(m.replacements)))
			m.known = pushPeer(m.known, r, maxKnow)
		}
		return
	}

	// no need to do anything here, as the peer is bumped when handling the pong
	m.log.Debugw("reverified",
		"peer", p,
	)
}

// peerToReverify returns the oldest peer, or nil if empty.
func (m *manager) peerToReverify() *mpeer {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if len(m.known) == 0 {
		return nil
	}
	// the last peer is the oldest
	return m.known[len(m.known)-1]
}

// updatePeer moves the peer with the given ID to the front of the list of managed peers.
// It returns true if a peer was bumped or false if there was no peer with that id
func (m *manager) updatePeer(update *peer.Peer) bool {
	id := update.ID()
	for i, p := range m.known {
		if p.ID() == id {
			if i > 1 {
				//  move it to the front
				copy(m.known[1:], m.known[:i])
				m.known[0] = p
			}
			p.updatePeer(update)
			p.verifiedCount++
			return true
		}
	}
	return false
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

	if containsPeer(m.known, p.ID()) {
		return false
	}
	m.log.Debugw("add discovered",
		"peer", p,
	)

	mp := wrapPeer(p)
	if len(m.known) >= maxKnow {
		return m.addReplacement(mp)
	}

	m.known = pushPeer(m.known, mp, maxKnow)
	return true
}

// addVerifiedPeer adds a new peer that has just been successfully pinged.
// It returns true, if the given peer was new and added, false otherwise.
func (m *manager) addVerifiedPeer(p *peer.Peer) bool {
	// never add the local peer
	if p.ID() == m.self() {
		return false
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// if already in the list, move it to the front
	if m.updatePeer(p) {
		return false
	}
	m.log.Debugw("add verified",
		"peer", p,
	)
	Events.PeerDiscovered.Trigger(&DiscoveredEvent{Peer: p})

	mp := wrapPeer(p)
	mp.verifiedCount = 1

	if len(m.known) >= maxKnow {
		return m.addReplacement(mp)
	}

	// new nodes are added to the front
	m.known = unshiftPeer(m.known, mp, maxKnow)
	return true
}

// getRandomPeers returns a list of randomly selected peers.
func (m *manager) getRandomPeers(n int, minVerified uint) []*peer.Peer {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if n > len(m.known) {
		n = len(m.known)
	}

	peers := make([]*peer.Peer, 0, n)
	for _, i := range rand.Perm(len(m.known)) {
		if len(peers) == n {
			break
		}

		mp := m.known[i]
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

	peers := make([]*mpeer, 0, len(m.known))
	for _, mp := range m.known {
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

	return containsPeer(m.known, id) || containsPeer(m.replacements, id)
}
