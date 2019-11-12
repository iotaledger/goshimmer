package peer

import (
	"bytes"
	"encoding/binary"
	"time"

	"github.com/iotaledger/goshimmer/packages/database"
)

const (
	// remove peers from DB, when the last received ping was older than this
	peerExpiration = 24 * time.Hour
	// interval in which expired peers are checked
	cleanupInterval = time.Hour

	keySeparator = ":"

	// These fields are stored per ID and address. Use nodeItemKey to create those keys.
	dbNodePing = "lastping"
	dbNodePong = "lastpong"
)

// DB is the peer database, storing previously seen peers and any collected
// properties of them.
type DB interface {
	// LastPing returns that property for the given peer ID and address.
	LastPing(id ID, address string) time.Time
	// UpdateLastPing updates that property for the given peer ID and address.
	UpdateLastPing(id ID, address string, t time.Time) error

	// LastPong returns that property for the given peer ID and address.
	LastPong(id ID, address string) time.Time
	// UpdateLastPong updates that property for the given peer ID and address.
	UpdateLastPong(id ID, address string, t time.Time) error

	// Close closes the DB.
	Close()
}

type persistentDB struct {
	db database.Database
}

func NewPersistentDB() DB {
	db, err := database.Get("peer")
	if err != nil {
		panic(err)
	}

	return &persistentDB{
		db: db,
	}
}

// Close closes the DB.
func (db *persistentDB) Close() {}

// nodeItemKey returns the database key for a node metadata field.
func nodeItemKey(id ID, address string, field string) []byte {
	return bytes.Join([][]byte{id.Bytes(), []byte(address), []byte(field)}, []byte(keySeparator))
}

// getInt64 retrieves an integer associated with a particular key.
func (db *persistentDB) getInt64(key []byte) int64 {
	blob, err := db.db.Get(key)
	if err != nil {
		return 0
	}
	val, read := binary.Varint(blob)
	if read <= 0 {
		return 0
	}
	return val
}

// setInt64 stores an integer in the given key.
func (db *persistentDB) setInt64(key []byte, n int64) error {
	blob := make([]byte, binary.MaxVarintLen64)
	blob = blob[:binary.PutVarint(blob, n)]
	return db.db.Set(key, blob)
}

// LastPing returns that property for the given peer ID and address.
func (db *persistentDB) LastPing(id ID, address string) time.Time {
	return time.Unix(db.getInt64(nodeItemKey(id, address, dbNodePing)), 0)
}

// UpdateLastPing updates that property for the given peer ID and address.
func (db *persistentDB) UpdateLastPing(id ID, address string, t time.Time) error {
	return db.setInt64(nodeItemKey(id, address, dbNodePing), t.Unix())
}

// LastPing returns that property for the given peer ID and address.
func (db *persistentDB) LastPong(id ID, address string) time.Time {
	return time.Unix(db.getInt64(nodeItemKey(id, address, dbNodePong)), 0)
}

// UpdateLastPing updates that property for the given peer ID and address.
func (db *persistentDB) UpdateLastPong(id ID, address string, t time.Time) error {
	return db.setInt64(nodeItemKey(id, address, dbNodePong), t.Unix())
}
