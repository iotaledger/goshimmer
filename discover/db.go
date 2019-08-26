package discover

import (
	"sync"
	"time"

	"github.com/wollac/autopeering/id"
	log "go.uber.org/zap"
)

const (
	// remove peers from DB, when the last received ping was older than this
	peerExpiration = 24 * time.Hour
	// interval in which expired peers are checked
	cleanupInterval = time.Hour
)

type DB struct {
	m   map[string]*dbPeer
	mu  sync.RWMutex
	log *log.SugaredLogger

	once    sync.Once
	wg      sync.WaitGroup
	closing chan struct{}
}

type dbPeer struct {
	Peer

	lastPing time.Time
	lastPong time.Time
}

func NewMapDB(log *log.SugaredLogger) *DB {
	db := &DB{
		m:       make(map[string]*dbPeer),
		log:     log,
		closing: make(chan struct{}),
	}

	return db
}

func (db *DB) Close() {
	db.log.Debugf("closing")

	close(db.closing)
	db.wg.Wait()
}

func (db *DB) ensureExpirer() {
	db.once.Do(func() {
		db.wg.Add(1)
		go db.expirer()
	})
}

func (db *DB) expirer() {
	defer db.wg.Done()

	tick := time.NewTicker(cleanupInterval)
	defer tick.Stop()

	db.expirePeers()
	for {
		select {
		case <-tick.C:
			db.expirePeers()
		case <-db.closing:
			return
		}
	}
}

func (db *DB) expirePeers() {
	threshold := time.Now().Add(-peerExpiration)

	db.mu.Lock()
	for id, entry := range db.m {
		if entry.lastPong.Before(threshold) {
			delete(db.m, id)
		}
	}
	db.mu.Unlock()
}

func (db *DB) getOrSet(id *id.Identity) *dbPeer {
	entry := db.m[id.StringID]
	if entry == nil {
		entry = &dbPeer{}
		db.m[id.StringID] = entry
	}
	return entry
}

func (db *DB) UpdatePeer(peer *Peer) {
	db.ensureExpirer()

	db.mu.Lock()
	entry := db.getOrSet(peer.Identity)
	entry.Peer = *peer
	db.mu.Unlock()
}

func (db *DB) Get(id *id.Identity) *Peer {
	db.mu.RLock()
	defer db.mu.RUnlock()

	return &db.m[id.StringID].Peer
}
