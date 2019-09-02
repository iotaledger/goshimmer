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
	queryInterval    = 20 * time.Second

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

		query     = time.NewTimer(0)
		queryNext chan time.Duration
	)
	defer reverify.Stop()
	defer query.Stop()

Loop:
	for {
		select {
		// on close, exit the loop
		case <-m.closing:
			break Loop

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

	m.bumpPeer(last.ID())

	m.log.Debugw("reverified",
		"peer", last,
		"count", last.verifiedCount,
	)
}

func (m *manager) doQuery(next chan<- time.Duration) {
	defer func() { next <- queryInterval }()

	ps := m.peersToQuery()

	for _, p := range ps {
		r, err := m.net.requestPeers(unwrapPeer(p))
		if err != nil {
			m.log.Debugw("requestPeers failed",
				"peer", nil,
				"err", err,
			)
			continue
		}

		var added int
		for _, rp := range r {
			if m.addDiscoveredPeer(rp) {
				added++
			}
		}
		m.log.Debugw("requestPeers",
			"peer", nil,
			"new", added,
		)
	}

}

func (m *manager) peersToQuery() []*mpeer {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if len(m.known) == 0 {
		return nil
	}
	if m.known[0].verifiedCount < 1 {
		return nil
	}

	return []*mpeer{m.known[0]}
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
// It returns true if a peer was bumped or false if there was no peer with that id
func (m *manager) bumpPeer(id peer.ID) bool {
	for i, p := range m.known {
		if p.ID() == id {
			// update and move it to the front
			copy(m.known[1:], m.known[:i])
			m.known[0] = p
			p.verifiedCount++
			return true
		}
	}
	return false
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

func (m *manager) addReplacement(p *mpeer) bool {
	if containsPeer(m.replacements, p.ID()) {
		return false // already in the list
	}
	m.replacements = pushPeer(m.replacements, p, maxReplacements)
	return true
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

	m.known = append(m.known, mp)
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
	if m.bumpPeer(p.ID()) {
		return false
	}
	m.log.Debugw("add verified",
		"peer", p,
	)

	mp := wrapPeer(p)
	mp.verifiedCount = 1

	// new nodes are added to the front
	m.known = pushPeer(m.known, mp, maxKnow)
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
