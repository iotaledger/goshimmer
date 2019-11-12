package peer

import (
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// mapDB is a simple implementation of DB using a map.
type mapDB struct {
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

// NewMemoryDB creates a new DB that uses a GO map.
func NewMemoryDB(log *zap.SugaredLogger) DB {
	db := &mapDB{
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
func (db *mapDB) Close() {
	db.log.Debugf("closing")

	close(db.closing)
	db.wg.Wait()
}

// peerPropKey returns the DB key to store additional peer properties.
func peerPropKey(id ID, address string) string {
	return strings.Join([]string{string(id.Bytes()), address}, keySeparator)
}

// LastPing returns that property for the given peer ID and address.
func (db *mapDB) LastPing(id ID, address string) time.Time {
	key := peerPropKey(id, address)

	db.mutex.RLock()
	peerPropEntry := db.m[key]
	db.mutex.RUnlock()

	return time.Unix(peerPropEntry.lastPing, 0)
}

// UpdateLastPing updates that property for the given peer ID and address.
func (db *mapDB) UpdateLastPing(id ID, address string, t time.Time) error {
	key := peerPropKey(id, address)

	db.mutex.Lock()
	peerPropEntry := db.m[key]
	peerPropEntry.lastPing = t.Unix()
	db.m[key] = peerPropEntry
	db.mutex.Unlock()

	return nil
}

// LastPong returns that property for the given peer ID and address.
func (db *mapDB) LastPong(id ID, address string) time.Time {
	key := peerPropKey(id, address)

	db.mutex.RLock()
	peerPropEntry := db.m[key]
	db.mutex.RUnlock()

	return time.Unix(peerPropEntry.lastPong, 0)
}

// UpdateLastPong updates that property for the given peer ID and address.
func (db *mapDB) UpdateLastPong(id ID, address string, t time.Time) error {
	key := peerPropKey(id, address)

	db.mutex.Lock()
	peerPropEntry := db.m[key]
	peerPropEntry.lastPong = t.Unix()
	db.m[key] = peerPropEntry
	db.mutex.Unlock()

	return nil
}

func (db *mapDB) expirer() {
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

func (db *mapDB) expirePeers() {
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
