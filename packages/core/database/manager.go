package database

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"

	"github.com/pkg/errors"
	"github.com/zyedidia/generic/cache"

	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/ioutils"
	"github.com/iotaledger/hive.go/runtime/options"
)

// region Manager //////////////////////////////////////////////////////////////////////////////////////////////////////

type Manager struct {
	permanentStorage kvstore.KVStore
	permanentBaseDir string

	openDBs         *cache.Cache[slot.Index, *dbInstance]
	bucketedBaseDir string
	dbSizes         *shrinkingmap.ShrinkingMap[slot.Index, int64]
	openDBsMutex    sync.Mutex

	maxPruned      slot.Index
	maxPrunedMutex sync.RWMutex

	// The granularity of the DB instances (i.e. how many buckets/slots are stored in one DB).
	optsGranularity int64
	optsBaseDir     string
	optsDBProvider  DBProvider
	optsMaxOpenDBs  int
}

func NewManager(version Version, opts ...options.Option[Manager]) *Manager {
	m := options.Apply(&Manager{
		maxPruned:       -1,
		optsGranularity: 10,
		optsBaseDir:     "db",
		optsDBProvider:  NewMemDB,
		optsMaxOpenDBs:  10,
	}, opts, func(m *Manager) {
		m.bucketedBaseDir = filepath.Join(m.optsBaseDir, "pruned")
		m.permanentBaseDir = filepath.Join(m.optsBaseDir, "permanent")
		db, err := m.optsDBProvider(m.permanentBaseDir)
		if err != nil {
			panic(err)
		}
		m.permanentStorage = db.NewStore()

		m.openDBs = cache.New[slot.Index, *dbInstance](m.optsMaxOpenDBs)
		m.openDBs.SetEvictCallback(func(baseIndex slot.Index, db *dbInstance) {
			err := db.instance.Close()
			if err != nil {
				panic(err)
			}
			size, err := dbPrunableDirectorySize(m.bucketedBaseDir, baseIndex)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}

			m.dbSizes.Set(baseIndex, size)
		})

		m.dbSizes = shrinkingmap.New[slot.Index, int64]()
	})

	if err := m.checkVersion(version); err != nil {
		panic(err)
	}

	return m
}

// checkVersion checks whether the database is compatible with the current schema version.
// also automatically sets the version if the database is new.
func (m *Manager) checkVersion(version Version) error {
	entry, err := m.permanentStorage.Get(dbVersionKey)
	if errors.Is(err, kvstore.ErrKeyNotFound) {
		// set the version in an empty DB
		return m.permanentStorage.Set(dbVersionKey, lo.PanicOnErr(version.Bytes()))
	}
	if err != nil {
		return err
	}
	if len(entry) == 0 {
		return errors.Errorf("no database version was persisted")
	}
	var storedVersion Version
	if _, err := storedVersion.FromBytes(entry); err != nil {
		return err
	}
	if storedVersion != version {
		return errors.Errorf("incompatible database versions: supported version: %d, version of database: %d", version, storedVersion)
	}
	return nil
}

func (m *Manager) PermanentStorage() kvstore.KVStore {
	return m.permanentStorage
}

func (m *Manager) RestoreFromDisk() (latestBucketIndex slot.Index) {
	dbInfos := getSortedDBInstancesFromDisk(m.bucketedBaseDir)

	// TODO: what to do if dbInfos is empty? -> start with a fresh DB?

	for _, dbInfo := range dbInfos {
		size, err := dbPrunableDirectorySize(m.bucketedBaseDir, dbInfo.baseIndex)
		if err != nil {
			panic(err)
		}
		m.dbSizes.Set(dbInfo.baseIndex, size)
	}

	m.maxPrunedMutex.Lock()
	m.maxPruned = dbInfos[len(dbInfos)-1].baseIndex - 1
	m.maxPrunedMutex.Unlock()

	for _, dbInfo := range dbInfos {
		dbIndex := dbInfo.baseIndex
		var healthy bool
		for bucketIndex := dbIndex + slot.Index(m.optsGranularity) - 1; bucketIndex >= dbIndex; bucketIndex-- {
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

func (m *Manager) MaxPrunedSlot() slot.Index {
	m.maxPrunedMutex.RLock()
	defer m.maxPrunedMutex.RUnlock()

	return m.maxPruned
}

// IsTooOld checks if the Block associated with the given id is too old (in a pruned slot).
func (m *Manager) IsTooOld(index slot.Index) (isTooOld bool) {
	m.maxPrunedMutex.RLock()
	defer m.maxPrunedMutex.RUnlock()

	return index <= m.maxPruned
}

func (m *Manager) Get(index slot.Index, realm kvstore.Realm) kvstore.KVStore {
	if m.IsTooOld(index) {
		return nil
	}

	bucket := m.getBucket(index)
	withRealm, err := bucket.WithExtendedRealm(realm)
	if err != nil {
		panic(err)
	}

	return withRealm
}

func (m *Manager) Flush(index slot.Index) {
	// Flushing works on DB level
	db := m.getDBInstance(index)
	err := db.store.Flush()
	if err != nil {
		panic(err)
	}

	// Mark as healthy.
	bucket := m.getBucket(index)
	err = bucket.Set(healthKey, []byte{1})
	if err != nil {
		panic(err)
	}
}

func (m *Manager) PruneUntilSlot(index slot.Index) {
	var baseIndexToPrune slot.Index
	if m.computeDBBaseIndex(index)+slot.Index(m.optsGranularity)-1 == index {
		// Upper bound of the DB instance should be pruned. So we can delete the entire DB file.
		baseIndexToPrune = index
	} else {
		baseIndexToPrune = m.computeDBBaseIndex(index) - 1
	}

	currentPrunedIndex := m.setMaxPruned(baseIndexToPrune)
	for currentPrunedIndex+slot.Index(m.optsGranularity) <= baseIndexToPrune {
		currentPrunedIndex += slot.Index(m.optsGranularity)
		m.prune(currentPrunedIndex)
	}
}

func (m *Manager) setMaxPruned(index slot.Index) (previous slot.Index) {
	m.maxPrunedMutex.Lock()
	defer m.maxPrunedMutex.Unlock()

	if previous = m.maxPruned; previous >= index {
		return
	}

	m.maxPruned = index
	return
}

func (m *Manager) Shutdown() {
	m.openDBsMutex.Lock()
	defer m.openDBsMutex.Unlock()

	m.openDBs.Each(func(index slot.Index, db *dbInstance) {
		err := db.instance.Close()
		if err != nil {
			panic(err)
		}
	})

	err := m.permanentStorage.Close()
	if err != nil {
		panic(err)
	}
}

// TotalStorageSize returns the combined size of the permanent and prunable storage.
func (m *Manager) TotalStorageSize() int64 {
	return m.PermanentStorageSize() + m.PrunableStorageSize()
}

// PermanentStorageSize returns the size of the permanent storage.
func (m *Manager) PermanentStorageSize() int64 {
	size, err := dbDirectorySize(m.permanentBaseDir)
	if err != nil {
		fmt.Println("dbDirectorySize failed for", m.permanentBaseDir, ":", err)
		return 0
	}
	return size
}

// PrunableStorageSize returns the size of the prunable storage containing all db instances.
func (m *Manager) PrunableStorageSize() int64 {
	m.openDBsMutex.Lock()
	defer m.openDBsMutex.Unlock()

	// Sum up all the evicted databases
	var sum int64
	m.dbSizes.ForEach(func(index slot.Index, i int64) bool {
		sum += i
		return true
	})

	// Add up all the open databases
	m.openDBs.Each(func(key slot.Index, val *dbInstance) {
		size, err := dbPrunableDirectorySize(m.bucketedBaseDir, key)
		if err != nil {
			fmt.Println("dbPrunableDirectorySize failed for", m.bucketedBaseDir, key, ":", err)
			return
		}
		sum += size
	})

	return sum
}

// getDBInstance returns the DB instance for the given baseIndex or creates a new one if it does not yet exist.
// DBs are created as follows where each db is located in m.basedir/<starting baseIndex>/
// (assuming a bucket granularity=2):
//
//	baseIndex 0 -> db 0
//	baseIndex 1 -> db 0
//	baseIndex 2 -> db 2
func (m *Manager) getDBInstance(index slot.Index) (db *dbInstance) {
	baseIndex := m.computeDBBaseIndex(index)
	m.openDBsMutex.Lock()
	defer m.openDBsMutex.Unlock()

	// check if exists again, as other goroutine might have created it in parallel
	db, exists := m.openDBs.Get(baseIndex)
	if !exists {
		db = m.createDBInstance(baseIndex)

		// Remove the cached db size since we will open the db
		m.dbSizes.Delete(baseIndex)
		m.openDBs.Put(baseIndex, db)
	}

	return db
}

// getBucket returns the bucket for the given baseIndex or creates a new one if it does not yet exist.
// A bucket is marked as dirty by default.
// Buckets are created as follows (assuming a bucket granularity=2):
//
//	baseIndex 0 -> db 0 / bucket 0
//	baseIndex 1 -> db 0 / bucket 1
//	baseIndex 2 -> db 2 / bucket 2
//	baseIndex 3 -> db 2 / bucket 3
func (m *Manager) getBucket(index slot.Index) (bucket kvstore.KVStore) {
	_, bucket = m.getDBAndBucket(index)
	return bucket
}

func (m *Manager) getDBAndBucket(index slot.Index) (db *dbInstance, bucket kvstore.KVStore) {
	db = m.getDBInstance(index)
	return db, m.createBucket(db, index)
}

// createDBInstance creates a new DB instance for the given baseIndex.
// If a folder/DB for the given baseIndex already exists, it is opened.
func (m *Manager) createDBInstance(index slot.Index) (newDBInstance *dbInstance) {
	db, err := m.optsDBProvider(dbPathFromIndex(m.bucketedBaseDir, index))
	if err != nil {
		panic(err)
	}

	return &dbInstance{
		index:    index,
		instance: db,
		store:    db.NewStore(),
	}
}

// createBucket creates a new bucket for the given baseIndex. It uses the baseIndex as a realm on the underlying DB.
func (m *Manager) createBucket(db *dbInstance, index slot.Index) (bucket kvstore.KVStore) {
	bucket, err := db.store.WithExtendedRealm(indexToRealm(index))
	if err != nil {
		panic(err)
	}
	return bucket
}

func (m *Manager) computeDBBaseIndex(index slot.Index) slot.Index {
	return index / slot.Index(m.optsGranularity) * slot.Index(m.optsGranularity)
}

func (m *Manager) prune(index slot.Index) {
	dbBaseIndex := m.computeDBBaseIndex(index)
	m.removeDBInstance(dbBaseIndex)
}

func (m *Manager) removeDBInstance(dbBaseIndex slot.Index) {
	m.openDBsMutex.Lock()
	defer m.openDBsMutex.Unlock()

	db, exists := m.openDBs.Get(dbBaseIndex)
	if exists {
		err := db.instance.Close()
		if err != nil {
			panic(err)
		}
		m.openDBs.Remove(dbBaseIndex)
	}

	if err := os.RemoveAll(dbPathFromIndex(m.bucketedBaseDir, dbBaseIndex)); err != nil {
		panic(err)
	}

	// Delete the db size since we pruned the whole directory
	m.dbSizes.Delete(dbBaseIndex)
}

func (m *Manager) removeBucket(bucket kvstore.KVStore) {
	err := bucket.Clear()
	if err != nil {
		panic(err)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// WithGranularity sets the granularity of the DB instances (i.e. how many buckets/slots are stored in one DB).
// It thus also has an impact on how fine-grained buckets/slots can be pruned.
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// types ///////////////////////////////////////////////////////////////////////////////////////////////////////////////

// DBProvider is a function that creates a new DB instance.
type DBProvider func(dirname string) (DB, error)

type dbInstance struct {
	index    slot.Index
	instance DB              // actual DB instance on disk within folder index
	store    kvstore.KVStore // KVStore that is used to access the DB instance
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// utils ///////////////////////////////////////////////////////////////////////////////////////////////////////////////

var healthKey = []byte("bucket_health")

var dbVersionKey = []byte("db_version")

// indexToRealm converts an baseIndex to a realm with some shifting magic.
func indexToRealm(index slot.Index) kvstore.Realm {
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

func dbPathFromIndex(base string, index slot.Index) string {
	return filepath.Join(base, strconv.FormatInt(int64(index), 10))
}

type dbInstanceFileInfo struct {
	baseIndex slot.Index
	path      string
}

func getSortedDBInstancesFromDisk(baseDir string) (dbInfos []*dbInstanceFileInfo) {
	files, err := os.ReadDir(baseDir)
	if err != nil {
		panic(err)
	}

	files = lo.Filter(files, func(e os.DirEntry) bool { return e.IsDir() })
	dbInfos = lo.Map(files, func(e os.DirEntry) *dbInstanceFileInfo {
		atoi, convErr := strconv.Atoi(e.Name())
		if convErr != nil {
			return nil
		}
		return &dbInstanceFileInfo{
			baseIndex: slot.Index(atoi),
			path:      filepath.Join(baseDir, e.Name()),
		}
	})
	dbInfos = lo.Filter(dbInfos, func(info *dbInstanceFileInfo) bool { return info != nil })

	sort.Slice(dbInfos, func(i, j int) bool {
		return dbInfos[i].baseIndex > dbInfos[j].baseIndex
	})

	return dbInfos
}

func dbPrunableDirectorySize(base string, index slot.Index) (int64, error) {
	return dbDirectorySize(dbPathFromIndex(base, index))
}

func dbDirectorySize(path string) (int64, error) {
	return ioutils.FolderSize(path)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
