package peer

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer/service"
	"github.com/iotaledger/goshimmer/packages/database"
	"go.uber.org/zap"
)

const (
	// remove peers from DB, when the last received ping was older than this
	peerExpiration = 24 * time.Hour
	// interval in which expired peers are checked
	cleanupInterval = time.Hour

	// number of peers used for bootstrapping
	seedCount = 10
	// time after which potential seed peers should expire
	seedExpiration = 5 * 24 * time.Hour
)

// DB is the peer database, storing previously seen peers and any collected
// properties of them.
type DB interface {
	// LocalPrivateKey returns the private key stored in the database or creates a new one.
	LocalPrivateKey() (PrivateKey, error)
	// LocalServices returns the services stored in the database or creates an empty services.
	LocalServices() (service.Service, error)
	// UpdateLocalServices updates the services stored in the database.
	UpdateLocalServices(services service.Service) error

	// Peer retrieves a peer from the database.
	Peer(id ID) *Peer
	// UpdatePeer updates a peer in the database.
	UpdatePeer(p *Peer) error
	// SeedPeers retrieves random nodes to be used as potential bootstrap peers.
	SeedPeers() []*Peer

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
	db  database.Database
	log *zap.SugaredLogger

	closeOnce sync.Once
}

// Keys in the node database.
const (
	dbNodePrefix  = "n:"     // Identifier to prefix node entries with
	dbLocalPrefix = "local:" // Identifier to prefix local entries

	// These fields are stored per ID and address. Use nodeFieldKey to create those keys.
	dbNodePing = "lastping"
	dbNodePong = "lastpong"

	// Local information is keyed by ID only. Use localFieldKey to create those keys.
	dbLocalKey      = "key"
	dbLocalServices = "services"
)

// NewPersistentDB creates a new persistent DB.
func NewPersistentDB(log *zap.SugaredLogger) DB {
	db, err := database.Get("peer")
	if err != nil {
		panic(err)
	}

	pDB := &persistentDB{
		db:  db,
		log: log,
	}
	pDB.start()

	return pDB
}

// Close closes the DB.
func (db *persistentDB) Close() {
	db.closeOnce.Do(func() {
		db.persistSeeds()
	})
}

func (db *persistentDB) start() {
	// get all peers in the DB
	peers := db.getPeers(0)

	for _, p := range peers {
		// if they dont have an associated pong, give them a grace period
		if db.LastPong(p.ID(), p.Address()).Unix() == 0 {
			err := db.setPeerWithTTL(p, cleanupInterval)
			if err != nil {
				db.log.Warnw("database error", "err", err)
			}
		}
	}
}

// persistSeeds assures that potential bootstrap peers are not garbage collected.
func (db *persistentDB) persistSeeds() {
	// randomly select potential bootstrap peers
	peers := randomSubset(db.getPeers(peerExpiration), seedCount)

	for _, p := range peers {
		err := db.setPeerWithTTL(p, seedExpiration)
		if err != nil {
			db.log.Warnw("database error", "err", err)
		}
	}
	db.log.Infof("%d bootstrap peers remain in DB", len(peers))
}

// nodeKey returns the database key for a node record.
func nodeKey(id ID) []byte {
	return append([]byte(dbNodePrefix), id.Bytes()...)
}

func splitNodeKey(key []byte) (id ID, rest []byte) {
	if !bytes.HasPrefix(key, []byte(dbNodePrefix)) {
		return ID{}, nil
	}
	item := key[len(dbNodePrefix):]
	copy(id[:], item[:len(id)])
	return id, item[len(id):]
}

// nodeFieldKey returns the database key for a node metadata field.
func nodeFieldKey(id ID, address string, field string) []byte {
	return bytes.Join([][]byte{nodeKey(id), []byte(address), []byte(field)}, []byte{':'})
}

func localFieldKey(field string) []byte {
	return append([]byte(dbLocalPrefix), []byte(field)...)
}

func parseInt64(blob []byte) int64 {
	val, read := binary.Varint(blob)
	if read <= 0 {
		return 0
	}
	return val
}

// getInt64 retrieves an integer associated with a particular key.
func (db *persistentDB) getInt64(key []byte) int64 {
	blob, err := db.db.Get(key)
	if err != nil {
		return 0
	}
	return parseInt64(blob)
}

// setInt64 stores an integer in the given key.
func (db *persistentDB) setInt64(key []byte, n int64) error {
	blob := make([]byte, binary.MaxVarintLen64)
	blob = blob[:binary.PutVarint(blob, n)]
	return db.db.SetWithTTL(key, blob, peerExpiration)
}

// LocalPrivateKey returns the private key stored in the database or creates a new one.
func (db *persistentDB) LocalPrivateKey() (PrivateKey, error) {
	key, err := db.db.Get(localFieldKey(dbLocalKey))
	if err == database.ErrKeyNotFound {
		key, err = generatePrivateKey()
		if err == nil {
			err = db.db.Set(localFieldKey(dbLocalKey), key)
		}
		return key, err
	}
	if err != nil {
		return nil, err
	}

	return key, nil
}

// LocalServices returns the services stored in the database or creates an empty services.
func (db *persistentDB) LocalServices() (service.Service, error) {
	key, err := db.db.Get(localFieldKey(dbLocalServices))
	if err == database.ErrKeyNotFound {
		return service.New(), nil
	}
	if err != nil {
		return nil, err
	}

	services, err := service.Unmarshal(key)
	if err != nil {
		return nil, err
	}

	return services, nil
}

// UpdateLocalServices updates the services stored in the database.
func (db *persistentDB) UpdateLocalServices(services service.Service) error {
	value, err := services.CreateRecord().CreateRecord().Marshal()
	if err != nil {
		return err
	}
	return db.db.Set(localFieldKey(dbLocalServices), value)
}

// LastPing returns that property for the given peer ID and address.
func (db *persistentDB) LastPing(id ID, address string) time.Time {
	return time.Unix(db.getInt64(nodeFieldKey(id, address, dbNodePing)), 0)
}

// UpdateLastPing updates that property for the given peer ID and address.
func (db *persistentDB) UpdateLastPing(id ID, address string, t time.Time) error {
	return db.setInt64(nodeFieldKey(id, address, dbNodePing), t.Unix())
}

// LastPing returns that property for the given peer ID and address.
func (db *persistentDB) LastPong(id ID, address string) time.Time {
	return time.Unix(db.getInt64(nodeFieldKey(id, address, dbNodePong)), 0)
}

// UpdateLastPing updates that property for the given peer ID and address.
func (db *persistentDB) UpdateLastPong(id ID, address string, t time.Time) error {
	return db.setInt64(nodeFieldKey(id, address, dbNodePong), t.Unix())
}

func (db *persistentDB) setPeerWithTTL(p *Peer, ttl time.Duration) error {
	data, err := p.Marshal()
	if err != nil {
		return err
	}
	return db.db.SetWithTTL(nodeKey(p.ID()), data, ttl)
}

func (db *persistentDB) UpdatePeer(p *Peer) error {
	return db.setPeerWithTTL(p, peerExpiration)
}

func parsePeer(data []byte) *Peer {
	p, err := Unmarshal(data)
	if err != nil {
		return nil
	}
	return p
}

func (db *persistentDB) Peer(id ID) *Peer {
	data, err := db.db.Get(nodeKey(id))
	if err != nil {
		return nil
	}
	return parsePeer(data)
}

func randomSubset(peers []*Peer, m int) []*Peer {
	if len(peers) <= m {
		return peers
	}

	result := make([]*Peer, 0, m)
	for i, p := range peers {
		if rand.Intn(len(peers)-i) < m-len(result) {
			result = append(result, p)
		}
	}
	return result
}

func (db *persistentDB) getPeers(maxAge time.Duration) []*Peer {
	peers := make([]*Peer, 0)
	now := time.Now()

	err := db.db.ForEachWithPrefix([]byte(dbNodePrefix), func(key []byte, value []byte) {
		id, rest := splitNodeKey(key)
		if len(rest) > 0 {
			return
		}

		p := parsePeer(value)
		if p == nil || p.ID() != id {
			return
		}
		if maxAge > 0 && now.Sub(db.LastPong(p.ID(), p.Address())) > maxAge {
			return
		}

		peers = append(peers, p)
	})
	if err != nil {
		return []*Peer{}
	}
	return peers
}

// SeedPeers retrieves random nodes to be used as potential bootstrap peers.
func (db *persistentDB) SeedPeers() []*Peer {
	// get all stored peers and select subset
	peers := db.getPeers(0)

	seeds := randomSubset(peers, seedCount)
	db.log.Infof("%d potential bootstrap peers restored form DB", len(seeds))

	return seeds
}
