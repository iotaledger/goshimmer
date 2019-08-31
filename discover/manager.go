package discover

import (
	"math/rand"
	"sync"
	"time"

	"github.com/wollac/autopeering/peer"
	"go.uber.org/zap"
)

const (
	reverifyInterval = 10 * time.Second
	reverifyTries    = 2

	maxKnow         = 100
	maxReplacements = 10
)

type network interface {
	self() peer.ID

	ping(*peer.Peer) error
	requestPeers(*peer.Peer) ([]*peer.Peer, error)
}

// mpeer represents a discovered peer with additional data.
// The fields of Peer may not be modified.
type mpeer struct {
	peer.Peer

	verifiedCount uint // how often that peer could be verified
}

func (p *mpeer) ID() peer.ID {
	return p.Peer.ID()
}

func (p *mpeer) String() string {
	return p.Peer.String()
}

func wrapPeer(p *peer.Peer) *mpeer {
	return &mpeer{Peer: *p}
}

func unwrapPeer(p *mpeer) *peer.Peer {
	return &p.Peer
}

type manager struct {
	mutex        sync.Mutex // protects  known and replacement
	known        []*mpeer
	replacements []*mpeer

	net network
	db  *peer.DB // peer database
	log *zap.SugaredLogger

	wg      sync.WaitGroup
	closing chan struct{}
}

func newManager(net network, db *peer.DB, boot []*peer.Peer, log *zap.SugaredLogger) *manager {
	m := &manager{
		known:        make([]*mpeer, 0, maxKnow),
		replacements: make([]*mpeer, 0, maxReplacements),
		net:          net,
		db:           db,
		log:          log,
		closing:      make(chan struct{}),
	}
	m.loadInitialPeers(boot)

	m.wg.Add(1)
	go m.loop()

	return m
}

func (m *manager) self() peer.ID {
	return m.net.self()
}

func (m *manager) close() {
	m.log.Debugf("closing")

	close(m.closing)
	m.wg.Wait()
}

func (m *manager) loop() {
	defer m.wg.Done()

	var (
		reverify     = time.NewTimer(0) // setting this to 0 will cause a trigger right away
		reverifyDone chan struct{}
	)
	defer reverify.Stop()

Loop:
	for {
		select {
		case <-reverify.C:
			// if there is no reverifyDone, this means doReverify is not running
			if reverifyDone == nil {
				reverifyDone = make(chan struct{})
				go m.doReverify(reverifyDone)
			}
		case <-reverifyDone:
			reverifyDone = nil
			reverify.Reset(reverifyInterval) // reverify again after the given interval
		case <-m.closing:
			break Loop
		}
	}

	// wait for the reverify to finish
	if reverifyDone != nil {
		<-reverifyDone
	}
}

// doReverify pings the oldest known peer.
func (m *manager) doReverify(done chan<- struct{}) {
	defer func() { done <- struct{}{} }() // always signal, when the function returns

	last := m.peerToReverify()
	if last == nil {
		return // nothing can be reverify
	}

	var err error
	for i := 0; i < reverifyTries && err != nil; i++ {
		err = m.net.ping(unwrapPeer(last))
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if err != nil {
		if len(m.replacements) == 0 {
			m.known = m.known[:len(m.known)-1] // pop back
		} else {
			var r *mpeer
			m.replacements, r = deletePeer(m.replacements, rand.Intn(len(m.replacements)))
			m.known[len(m.known)-1] = r
		}

		m.log.Debugw("remove dead",
			"peer", last,
			"err", err,
		)
		return
	}

	last.verifiedCount++
	m.bumpPeer(last.ID())

	m.log.Debugw("reverified",
		"peer", last,
		"count", last.verifiedCount,
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

// bumpPeer moves the peer with the given ID to the front of the list of managed peers.
// It returns the peer that was bumped, or nil if there was no peer with that id
func (m *manager) bumpPeer(id peer.ID) *mpeer {
	for i, p := range m.known {
		if p.ID() == id {
			// update and move it to the front
			copy(m.known[1:], m.known[:i])
			m.known[0] = p
			return p
		}
	}
	return nil
}

// pushPeer is a helper function that adds a new peer to the front of the list.
func pushPeer(list []*mpeer, p *mpeer, max int) []*mpeer {
	if len(list) < max {
		list = append(list, nil)
	}
	copy(list[1:], list)
	list[0] = p

	return list
}

// containsPeer returns true if a peer with the given ID is in the list.
func containsPeer(list []*mpeer, id peer.ID) bool {
	for _, p := range list {
		if p.ID() == id {
			return true
		}
	}
	return false
}

// deletePeer is a helper that deletes the peer with the given index from the list.
func deletePeer(list []*mpeer, i int) ([]*mpeer, *mpeer) {
	p := list[i]

	copy(list[i:], list[i+1:])
	list[len(list)-1] = nil

	return list[:len(list)-1], p
}

func (m *manager) addReplacement(p *mpeer) {
	if containsPeer(m.replacements, p.ID()) {
		return // already in the list
	}
	m.replacements = pushPeer(m.replacements, p, maxReplacements)
}

func (m *manager) loadInitialPeers(boot []*peer.Peer) {
	var peers []*peer.Peer
	// TODO: load seed peers from the database
	peers = append(peers, boot...)
	for _, peer := range peers {
		m.addDiscoveredPeer(peer)
	}
}

// addDiscoveredPeer adds a newly discovered peer that has never been verified or pinged yet.
func (m *manager) addDiscoveredPeer(p *peer.Peer) {
	// never add the local peer
	if p.ID() == m.self() {
		return
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if containsPeer(m.known, p.ID()) {
		return
	}
	m.log.Debugw("add discovered",
		"peer", p,
	)

	mp := wrapPeer(p)
	if len(m.known) < maxKnow {
		m.known = append(m.known, mp)
	} else {
		m.addReplacement(mp)
	}
}

// addVerifiedPeer adds a new peer that has just been successfully pinged.
func (m *manager) addVerifiedPeer(p *peer.Peer) {
	// never add the local peer
	if p.ID() == m.self() {
		return
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// if already in the list, move it to the front
	if mp := m.bumpPeer(p.ID()); mp != nil {
		mp.verifiedCount++
		return
	}
	m.log.Debugw("add verified",
		"peer", p,
	)

	mp := wrapPeer(p)
	mp.verifiedCount = 1

	// new nodes are added to the front
	m.known = pushPeer(m.known, mp, maxKnow)
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

// GetVerifiedPeers returns all the currently managed peers that have been verified at least once.
func (m *manager) GetVerifiedPeers() []*peer.Peer {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	peers := make([]*peer.Peer, 0, len(m.known))
	for _, mp := range m.known {
		if mp.verifiedCount == 0 {
			continue
		}
		peers = append(peers, unwrapPeer(mp))
	}

	return peers
}
