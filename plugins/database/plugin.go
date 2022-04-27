// Package database is a plugin that manages the RocksDB database (e.g. garbage collection).
package database

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/shutdown"
)

// PluginName is the name of the database plugin.
const PluginName = "Database"

var (
	// Plugin is the plugin instance of the database plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
	log    *logger.Logger

	db                database.DB
	cacheTimeProvider *database.CacheTimeProvider
	cacheProviderOnce sync.Once
)

type dependencies struct {
	dig.In
	Store kvstore.KVStore
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)

	Plugin.Events.Init.Hook(event.NewClosure[*node.InitEvent](func(initEvent *node.InitEvent) {
		if err := initEvent.Container.Provide(createStore); err != nil {
			Plugin.Panic(err)
		}
	}))
}

// CacheTimeProvider returns the cacheTimeProvider instance.
func CacheTimeProvider() *database.CacheTimeProvider {
	cacheProviderOnce.Do(createCacheTimeProvider)
	return cacheTimeProvider
}

func createCacheTimeProvider() {
	cacheTimeProvider = database.NewCacheTimeProvider(Parameters.ForceCacheTime)
}

func createStore() kvstore.KVStore {
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

	return db.NewStore()
}

func configure(_ *node.Plugin) {
	configureHealthStore(deps.Store)

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
func manageDBLifetime(ctx context.Context) {
	// we mark the database only as corrupted from within a background worker, which means
	// that we only mark it as dirty, if the node actually started up properly (meaning no termination
	// signal was received before all plugins loaded).
	MarkDatabaseUnhealthy()
	<-ctx.Done()
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
