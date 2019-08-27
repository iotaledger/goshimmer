package discover

import (
	"strings"
	"sync"
	"time"

	log "go.uber.org/zap"
)

const (
	// remove peers from DB, when the last received ping was older than this
	peerExpiration = 24 * time.Hour
	// interval in which expired peers are checked
	cleanupInterval = time.Hour
)

type DB struct {
	mutex sync.RWMutex
	m     map[string]dbPeer

	log *log.SugaredLogger

	once    sync.Once
	wg      sync.WaitGroup
	closing chan struct{}
}

type dbPeer struct {
	lastPing, lastPong int64
}

func NewMapDB(log *log.SugaredLogger) *DB {
	db := &DB{
		m:       make(map[string]dbPeer),
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
	threshold := time.Now().Add(-peerExpiration).Unix()

	db.mutex.Lock()
	for key, entry := range db.m {
		if entry.lastPong <= threshold {
			delete(db.m, key)
		}
	}
	db.mutex.Unlock()
}

func getDBKey(p *peer) string {
	return strings.Join([]string{p.Identity.StringID, p.Address}, ":")
}

func (db *DB) LastPing(p *peer) time.Time {
	db.ensureExpirer()
	key := getDBKey(p)

	db.mutex.RLock()
	entry := db.m[key]
	db.mutex.RUnlock()

	return time.Unix(entry.lastPing, 0)
}

func (db *DB) UpdateLastPing(p *peer, t time.Time) {
	key := getDBKey(p)

	db.mutex.Lock()
	entry := db.m[key]
	entry.lastPing = t.Unix()
	db.m[key] = entry
	db.mutex.Unlock()
}

func (db *DB) LastPong(p *peer) time.Time {
	db.ensureExpirer()
	key := getDBKey(p)

	db.mutex.RLock()
	entry := db.m[key]
	db.mutex.RUnlock()

	return time.Unix(entry.lastPong, 0)
}

func (db *DB) UpdateLastPong(p *peer, t time.Time) {
	key := getDBKey(p)

	db.mutex.Lock()
	entry := db.m[key]
	entry.lastPong = t.Unix()
	db.m[key] = entry
	db.mutex.Unlock()
}
