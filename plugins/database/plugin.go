// Package database is a plugin that manages the RocksDB database (e.g. garbage collection).
package database

import (
	"strconv"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/shutdown"
)

// PluginName is the name of the database plugin.
const PluginName = "Database"

var (
	// plugin is the plugin instance of the database plugin.
	plugin     *node.Plugin
	pluginOnce sync.Once
	log        *logger.Logger

	db                database.DB
	store             kvstore.KVStore
	cacheTimeProvider *database.CacheTimeProvider
	storeOnce         sync.Once
	cacheProviderOnce sync.Once
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	pluginOnce.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	})
	return plugin
}

// Store returns the KVStore instance.
func Store() kvstore.KVStore {
	storeOnce.Do(createStore)
	return store
}

// CacheTimeProvider  returns the cacheTimeProvider instance
func CacheTimeProvider() *database.CacheTimeProvider {
	cacheProviderOnce.Do(createCacheTimeProvider)
	return cacheTimeProvider
}

func createCacheTimeProvider() {
	cacheTimeProvider = database.NewCacheTimeProvider(Parameters.ForceCacheTime)
}

// StoreRealm is a factory method for a different realm backed by the KVStore instance.
func StoreRealm(realm kvstore.Realm) kvstore.KVStore {
	return Store().WithRealm(realm)
}

func createStore() {
	log = logger.NewLogger(PluginName)

	var err error
	if Parameters.InMemory {
		db, err = database.NewMemDB()
	} else {
		db, err = database.NewDB(Parameters.Directory)
	}
	if err != nil {
		log.Fatal("Unable to open the database, please delete the database folder. Error: %s", err)
	}

	store = db.NewStore()
}

func configure(_ *node.Plugin) {
	// assure that the store is initialized
	store := Store()
	configureHealthStore(store)

	if err := checkDatabaseVersion(healthStore); err != nil {
		if errors.Is(err, ErrDBVersionIncompatible) {
			log.Fatalf("The database scheme was updated. Please delete the database folder. %s", err)
		}
		log.Fatalf("Failed to check database version: %s", err)
	}

	if Parameters.Directory != "" {
		val, err := strconv.ParseBool(Parameters.Dirty)
		if err != nil {
			log.Warnf("Invalid database.dirty flag: %s", err)
		} else if val {
			MarkDatabaseUnhealthy()
		} else {
			MarkDatabaseHealthy()
		}
	}

	if IsDatabaseUnhealthy() {
		log.Fatal("The database is marked as not properly shutdown/corrupted, please delete the database folder and restart.")
	}

	// we open the database in the configure, so we must also make sure it's closed here
	if err := daemon.BackgroundWorker(PluginName, manageDBLifetime, shutdown.PriorityDatabase); err != nil {
		log.Fatalf("Failed to start as daemon: %s", err)
	}

	// run GC up on startup
	runDatabaseGC()
}

func run(*node.Plugin) {
	// placeholder
}

// manageDBLifetime takes care of managing the lifetime of the database. It marks the database as dirty up on
// startup and unmarks it up on shutdown. Up on shutdown it will run the db GC and then close the database.
func manageDBLifetime(shutdownSignal <-chan struct{}) {
	// we mark the database only as corrupted from within a background worker, which means
	// that we only mark it as dirty, if the node actually started up properly (meaning no termination
	// signal was received before all plugins loaded).
	MarkDatabaseUnhealthy()
	<-shutdownSignal
	runDatabaseGC()
	MarkDatabaseHealthy()
	log.Infof("Syncing database to disk...")
	if err := db.Close(); err != nil {
		log.Errorf("Failed to flush the database: %s", err)
	}
	log.Infof("Syncing database to disk... done")
}

func runDatabaseGC() {
	if !db.RequiresGC() {
		return
	}
	log.Info("Running database garbage collection...")
	s := time.Now()
	if err := db.GC(); err != nil {
		log.Warnf("Database garbage collection failed: %s", err)
		return
	}
	log.Infof("Database garbage collection done, took %v...", time.Since(s))
}
