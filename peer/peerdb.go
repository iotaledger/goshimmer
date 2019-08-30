package peer

import (
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	// remove peers from DB, when the last received ping was older than this
	peerExpiration = 24 * time.Hour
	// interval in which expired peers are checked
	cleanupInterval = time.Hour

	keySeparator = ":"
)

// DB is the peer database, storing previously seen peers and any collected
// properties of them.
type DB struct {
	mutex sync.RWMutex
	m     map[string]peerPropEntry

	log *zap.SugaredLogger

	wg      sync.WaitGroup
	closing chan struct{}
}

// peerPropEntry wraps additional peer properties that are stored.
type peerPropEntry struct {
	lastPing, lastPong int64
}

// NewMapDB creates a new DB that uses a GO map.
func NewMapDB(log *zap.SugaredLogger) *DB {
	db := &DB{
		m:       make(map[string]peerPropEntry),
		log:     log,
		closing: make(chan struct{}),
	}

	// start the expirer routine
	db.wg.Add(1)
	go db.expirer()

	return db
}

// Close closes the DB.
func (db *DB) Close() {
	db.log.Debugf("closing")

	close(db.closing)
	db.wg.Wait()
}

// peerPropKey returns the DB key to store additional peer properties.
func peerPropKey(id ID, address string) string {
	return strings.Join([]string{string(id.Bytes()), address}, keySeparator)
}

// LastPing returns that property for the given peer ID and address.
func (db *DB) LastPing(id ID, address string) time.Time {
	key := peerPropKey(id, address)

	db.mutex.RLock()
	peerPropEntry := db.m[key]
	db.mutex.RUnlock()

	return time.Unix(peerPropEntry.lastPing, 0)
}

// UpdateLastPing updates that property for the given peer ID and address.
func (db *DB) UpdateLastPing(id ID, address string, t time.Time) {
	key := peerPropKey(id, address)

	db.mutex.Lock()
	peerPropEntry := db.m[key]
	peerPropEntry.lastPing = t.Unix()
	db.m[key] = peerPropEntry
	db.mutex.Unlock()
}

// LastPong returns that property for the given peer ID and address.
func (db *DB) LastPong(id ID, address string) time.Time {
	key := peerPropKey(id, address)

	db.mutex.RLock()
	peerPropEntry := db.m[key]
	db.mutex.RUnlock()

	return time.Unix(peerPropEntry.lastPong, 0)
}

// UpdateLastPong updates that property for the given peer ID and address.
func (db *DB) UpdateLastPong(id ID, address string, t time.Time) {
	key := peerPropKey(id, address)

	db.mutex.Lock()
	peerPropEntry := db.m[key]
	peerPropEntry.lastPong = t.Unix()
	db.m[key] = peerPropEntry
	db.mutex.Unlock()
}

func (db *DB) expirer() {
	defer db.wg.Done()

	// the expiring isn't triggert right away, to give the bootstrapping the chance to use older nodes
	tick := time.NewTicker(cleanupInterval)
	defer tick.Stop()

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
	var (
		threshold = time.Now().Add(-peerExpiration).Unix()
		count     int
	)

	db.mutex.Lock()
	for key, peerPropEntry := range db.m {
		if peerPropEntry.lastPong <= threshold {
			delete(db.m, key)
			count++
		}
	}
	db.mutex.Unlock()

	db.log.Info("expired peers", "count", count)
}
