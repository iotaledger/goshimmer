package database

import (
	"path/filepath"
	"strconv"
	"sync"

	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

// TODO:
//  - prune old buckets/epochs
//  - flush epochs based on epoch commitments (link together via plugins)
//  - prevent GET access to old buckets/epochs; and if still available and accessed, for how long should DB be kept open?
//  - add functionality for permanent storage
//  - on node startup: check if buckets are healthy and remove all that are unhealthy -> possibly report latest bucket
//     to enable seamless startup after a crash/shutdown during an epoch

// region Manager //////////////////////////////////////////////////////////////////////////////////////////////////////

var healthKey = []byte("bucket_health")

type dbInstance struct {
	index    epoch.Index
	instance DB                              // actual DB instance on disk within folder index
	store    kvstore.KVStore                 // KVStore that is used to access the DB instance
	buckets  map[epoch.Index]kvstore.KVStore // buckets that are created within this instance
}

type Manager struct {
	baseDir string
	opts    *Options

	dbs *shrinkingmap.ShrinkingMap[epoch.Index, *dbInstance]

	mutex sync.Mutex
}

func NewManager(baseDirectory string, opts ...Option) *Manager {
	managerOpts := &Options{}
	managerOpts.apply(defaultOptions...)
	managerOpts.apply(opts...)

	m := &Manager{
		baseDir: baseDirectory,
		opts:    managerOpts,
		dbs:     shrinkingmap.New[epoch.Index, *dbInstance](),
	}

	return m
}

func (m *Manager) Get(index epoch.Index, realm kvstore.Realm) kvstore.KVStore {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	withRealm, err := m.getBucket(index).WithRealm(realm)
	if err != nil {
		panic(err)
	}

	return withRealm
}

func (m *Manager) Flush(index epoch.Index) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Flushing works on DB level
	db := m.getDBInstance(index)
	db.store.Flush()

	// Mark as healthy.
	bucket := m.getBucket(index)
	err := bucket.Set(healthKey, []byte{1})
	if err != nil {
		panic(err)
	}
}

func (m *Manager) Shutdown() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.dbs.ForEach(func(index epoch.Index, db *dbInstance) bool {
		db.instance.Close()
		return true
	})
}

// getDBInstance returns the DB instance for the given index or creates a new one if it does not yet exist.
// DBs are created as follows where each db is located in m.basedir/<starting index>/
// (assuming a bucket granularity=2):
//   index 0 -> db 0
//   index 1 -> db 0
//   index 2 -> db 2
func (m *Manager) getDBInstance(index epoch.Index) (db *dbInstance) {
	startingIndex := index / epoch.Index(m.opts.granularity) * epoch.Index(m.opts.granularity)
	db, exists := m.dbs.Get(startingIndex)
	if !exists {
		db = m.createDBInstance(startingIndex)
		m.dbs.Set(startingIndex, db)
	}

	return db
}

// getBucket returns the bucket for the given index or creates a new one if it does not yet exist.
// A bucket is marked as dirty by default.
// Buckets are created as follows (assuming a bucket granularity=2):
//   index 0 -> db 0 / bucket 0
//   index 1 -> db 0 / bucket 1
//   index 2 -> db 2 / bucket 2
//   index 3 -> db 2 / bucket 3
func (m *Manager) getBucket(index epoch.Index) (bucket kvstore.KVStore) {
	db := m.getDBInstance(index)

	bucket, exists := db.buckets[index]
	if !exists {
		bucket = m.createBucket(db, index)
		db.buckets[index] = bucket
	}

	return bucket
}

// createDBInstance creates a new DB instance for the given index.
// If a folder/DB for the given index already exists, it is opened.
func (m *Manager) createDBInstance(index epoch.Index) (newDBInstance *dbInstance) {
	db, err := m.opts.dbProvider(filepath.Join(m.baseDir, strconv.FormatInt(int64(index), 10)))
	if err != nil {
		panic(err)
	}

	return &dbInstance{
		index:    index,
		instance: db,
		store:    db.NewStore(),
		buckets:  make(map[epoch.Index]kvstore.KVStore),
	}
}

// createBucket creates a new bucket for the given index. It uses the index as a realm on the underlying DB.
func (m *Manager) createBucket(db *dbInstance, index epoch.Index) (bucket kvstore.KVStore) {
	bucket, err := db.store.WithRealm(indexToRealm(index))
	if err != nil {
		panic(err)
	}
	return bucket
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// the default options applied to Manager.
var defaultOptions = []Option{
	WithGranularity(10),
	WithDBProvider(NewDB),
}

// Options define options for Manager.
type Options struct {
	// The granularity of the DB instances (i.e. how many buckets/epochs are stored in one DB).
	granularity int64
	dbProvider  DBProvider
}

// applies the given Option.
func (o *Options) apply(opts ...Option) {
	for _, opt := range opts {
		opt(o)
	}
}

// WithGranularity sets the granularity of the DB instances (i.e. how many buckets/epochs are stored in one DB).
// It thus also has an impact on how fine-grained buckets/epochs can be pruned.
func WithGranularity(granularity int64) Option {
	return func(opts *Options) {
		opts.granularity = granularity
	}
}

// WithDBProvider sets the DB provider that is used to create new DB instances.
func WithDBProvider(provider DBProvider) Option {
	return func(opts *Options) {
		opts.dbProvider = provider
	}
}

// Option is a function setting an Options option.
type Option func(opts *Options)

// DBProvider is a function that creates a new DB instance.
type DBProvider func(dirname string) (DB, error)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// indexToRealm converts an index to a realm with some shifting magic.
func indexToRealm(index epoch.Index) kvstore.Realm {
	return []byte{
		byte(0xff & index),
		byte(0xff & (index >> 8)),
		byte(0xff & (index >> 16)),
		byte(0xff & (index >> 24)),
		byte(0xff & (index >> 32)),
		byte(0xff & (index >> 40)),
		byte(0xff & (index >> 48)),
		byte(0xff & (index >> 54)),
	}
}
