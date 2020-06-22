// Package database is a plugin that manages the badger database (e.g. garbage collection).
package database

import (
	"errors"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/database/prefix"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the database plugin.
const PluginName = "Database"

var (
	// Plugin is the plugin instance of the database plugin.
	Plugin = node.NewPlugin(PluginName, node.Enabled, configure)
	log    *logger.Logger

	db        database.DB
	store     kvstore.KVStore
	storeOnce sync.Once
)

// Store returns the KVStore instance.
func Store() kvstore.KVStore {
	storeOnce.Do(createStore)
	return store
}

// StoreRealm is a factory method for a different realm backed by the KVStore instance.
func StoreRealm(realm kvstore.Realm) kvstore.KVStore {
	return Store().WithRealm(realm)
}

func createStore() {
	log = logger.NewLogger(PluginName)

	var err error
	if config.Node.GetBool(CfgDatabaseInMemory) {
		db, err = database.NewMemDB()
	} else {
		dbDir := config.Node.GetString(CfgDatabaseDir)
		db, err = database.NewDB(dbDir)
	}
	if err != nil {
		log.Fatal(err)
	}

	store = db.NewStore()
}

func configure(_ *node.Plugin) {
	// assure that the store is initialized
	store := Store()
	configureHealthStore(store)

	if err := checkDatabaseVersion(store.WithRealm([]byte{prefix.DBPrefixDatabaseVersion})); err != nil {
		if errors.Is(err, ErrDBVersionIncompatible) {
			log.Panicf("The database scheme was updated. Please delete the database folder.\n%s", err)
		}
		log.Panicf("Failed to check database version: %s", err)
	}

	if IsDatabaseUnhealthy() {
		log.Panic("The database is marked as not properly shutdown/corrupted, please delete the database folder and restart.")
	}

	// we mark the database only as corrupted from within a background worker, which means
	// that we only mark it as dirty, if the node actually started up properly (meaning no termination
	// signal was received before all plugins loaded).
	if err := daemon.BackgroundWorker("[Database Health]", func(shutdownSignal <-chan struct{}) {
		MarkDatabaseUnhealthy()
	}); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}

	// we open the database in the configure, so we must also make sure it's closed here
	if err := daemon.BackgroundWorker(PluginName, closeDB, shutdown.PriorityDatabase); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}

	// run GC up on startup
	runDatabaseGC()
}

func closeDB(shutdownSignal <-chan struct{}) {
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
