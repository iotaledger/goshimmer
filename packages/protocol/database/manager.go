package database

import (
	"context"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"

	"github.com/iotaledger/hive.go/core/byteutils"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/timeutil"
	"github.com/zyedidia/generic/cache"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

// region Manager //////////////////////////////////////////////////////////////////////////////////////////////////////

var healthKey = []byte("bucket_health")

type dbInstance struct {
	index    epoch.Index
	instance DB              // actual DB instance on disk within folder index
	store    kvstore.KVStore // KVStore that is used to access the DB instance
}

type Manager struct {
	openDBs *cache.Cache[epoch.Index, *dbInstance]
	cleaner *timeutil.Ticker
	ctx     context.Context
	mutex   sync.Mutex

	lastPrunedIndex epoch.Index

	// The granularity of the DB instances (i.e. how many buckets/epochs are stored in one DB).
	optsGranularity int64
	optsBaseDir     string
	optsDBProvider  DBProvider
	optsMaxOpenDBs  int
}

func NewManager(ctx context.Context, opts ...options.Option[Manager]) *Manager {
	return options.Apply(&Manager{
		ctx: ctx,

		optsGranularity: 10,
		optsBaseDir:     "db",
		optsDBProvider:  NewMemDB,
		optsMaxOpenDBs:  10,
	}, opts, func(m *Manager) {
		m.openDBs = cache.New[epoch.Index, *dbInstance](m.optsMaxOpenDBs)
		m.openDBs.SetEvictCallback(func(index epoch.Index, db *dbInstance) {
			db.instance.Close()
		})
	})
}

func (m *Manager) RestoreFromDisk() (latestBucketIndex epoch.Index) {
	dbInfos := getSortedDBInstancesFromDisk(m.optsBaseDir)

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.lastPrunedIndex = dbInfos[len(dbInfos)-1].index - 1

	for _, dbInfo := range dbInfos {
		dbIndex := dbInfo.index
		var healthy bool
		for bucketIndex := dbIndex + epoch.Index(m.optsGranularity) - 1; bucketIndex >= dbIndex; bucketIndex-- {
			bucket := m.getBucket(bucketIndex)
			healthy = lo.PanicOnErr(bucket.Has(healthKey))
			if healthy {
				return bucketIndex
			}

			m.removeBucket(bucket)
		}
		m.removeDBInstance(dbIndex)
	}

	return 0
}

func (m *Manager) Get(index epoch.Index, realm kvstore.Realm) kvstore.KVStore {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if index <= m.lastPrunedIndex {
		return nil
	}

	bucket := m.getBucket(index)
	withRealm, err := bucket.WithRealm(byteutils.ConcatBytes(bucket.Realm(), realm))
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

func (m *Manager) PruneUntilEpoch(index epoch.Index) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var indexToPrune epoch.Index
	if m.computeDBBaseIndex(index)+epoch.Index(m.optsGranularity)-1 == index {
		// Upper bound of the DB instance should be pruned. So we can delete the entire DB file.
		indexToPrune = index
	} else {
		indexToPrune = m.computeDBBaseIndex(index) - 1
	}

	for m.lastPrunedIndex < indexToPrune {
		m.prune(m.lastPrunedIndex)

		if m.lastPrunedIndex == 0 {
			m.lastPrunedIndex = epoch.Index(m.optsGranularity) - 1
			continue
		}
		m.lastPrunedIndex = m.lastPrunedIndex + epoch.Index(m.optsGranularity)
	}
}

func (m *Manager) prune(index epoch.Index) {
	fmt.Println("prune", index)
	dbBaseIndex := m.computeDBBaseIndex(index)
	m.removeDBInstance(dbBaseIndex)
}

func (m *Manager) removeDBInstance(dbBaseIndex epoch.Index) {
	_, exists := m.openDBs.Get(dbBaseIndex)
	if exists {
		m.openDBs.Remove(dbBaseIndex)
	}
	if err := os.RemoveAll(dbPathFromIndex(m.optsBaseDir, dbBaseIndex)); err != nil {
		panic(err)
	}
}

func (m *Manager) removeBucket(bucket kvstore.KVStore) {
	err := bucket.Clear()
	if err != nil {
		panic(err)
	}
}

func (m *Manager) Shutdown() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.cleaner != nil {
		m.cleaner.WaitForGracefulShutdown()
	}

	m.openDBs.Each(func(index epoch.Index, db *dbInstance) {
		db.instance.Close()
	})
}

// getDBInstance returns the DB instance for the given index or creates a new one if it does not yet exist.
// DBs are created as follows where each db is located in m.basedir/<starting index>/
// (assuming a bucket granularity=2):
//   index 0 -> db 0
//   index 1 -> db 0
//   index 2 -> db 2
func (m *Manager) getDBInstance(index epoch.Index) (db *dbInstance) {
	startingIndex := m.computeDBBaseIndex(index)
	db, exists := m.openDBs.Get(startingIndex)
	if !exists {
		db = m.createDBInstance(startingIndex)
		m.openDBs.Put(startingIndex, db)
	}

	return db
}

func (m *Manager) computeDBBaseIndex(index epoch.Index) epoch.Index {
	startingIndex := index / epoch.Index(m.optsGranularity) * epoch.Index(m.optsGranularity)
	return startingIndex
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

	return m.createBucket(db, index)
}

func (m *Manager) getDBAndBucket(index epoch.Index) (db *dbInstance, bucket kvstore.KVStore) {
	db = m.getDBInstance(index)
	return db, m.createBucket(db, index)
}

// createDBInstance creates a new DB instance for the given index.
// If a folder/DB for the given index already exists, it is opened.
func (m *Manager) createDBInstance(index epoch.Index) (newDBInstance *dbInstance) {
	db, err := m.optsDBProvider(dbPathFromIndex(m.optsBaseDir, index))
	if err != nil {
		panic(err)
	}

	return &dbInstance{
		index:    index,
		instance: db,
		store:    db.NewStore(),
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

// WithGranularity sets the granularity of the DB instances (i.e. how many buckets/epochs are stored in one DB).
// It thus also has an impact on how fine-grained buckets/epochs can be pruned.
func WithGranularity(granularity int64) options.Option[Manager] {
	return func(m *Manager) {
		m.optsGranularity = granularity
	}
}

// WithDBProvider sets the DB provider that is used to create new DB instances.
func WithDBProvider(provider DBProvider) options.Option[Manager] {
	return func(m *Manager) {
		m.optsDBProvider = provider
	}
}

// WithBaseDir sets the base directory to store the DB to disk.
func WithBaseDir(baseDir string) options.Option[Manager] {
	return func(m *Manager) {
		m.optsBaseDir = baseDir
	}
}

// WithMaxOpenDBs sets the maximum concurrently open DBs.
func WithMaxOpenDBs(optsMaxOpenDBs int) options.Option[Manager] {
	return func(m *Manager) {
		m.optsMaxOpenDBs = optsMaxOpenDBs
	}
}

// DBProvider is a function that creates a new DB instance.
type DBProvider func(dirname string) (DB, error)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// utils ///////////////////////////////////////////////////////////////////////////////////////////////////////////////

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

func dbPathFromIndex(base string, index epoch.Index) string {
	return filepath.Join(base, strconv.FormatInt(int64(index), 10))
}

type dbInstanceFileInfo struct {
	index epoch.Index
	path  string
}

func getSortedDBInstancesFromDisk(baseDir string) (dbInfos []*dbInstanceFileInfo) {
	files, err := ioutil.ReadDir(baseDir)
	if err != nil {
		panic(err)
	}

	files = lo.Filter(files, func(f fs.FileInfo) bool { return f.IsDir() })
	dbInfos = lo.Map(files, func(f fs.FileInfo) *dbInstanceFileInfo {
		atoi, convErr := strconv.Atoi(f.Name())
		if convErr != nil {
			return nil
		}
		return &dbInstanceFileInfo{
			index: epoch.Index(atoi),
			path:  filepath.Join(baseDir, f.Name()),
		}
	})
	dbInfos = lo.Filter(dbInfos, func(info *dbInstanceFileInfo) bool { return info != nil })

	sort.Slice(dbInfos, func(i, j int) bool {
		return dbInfos[i].index > dbInfos[j].index
	})

	return dbInfos
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
