// Wrapper for hive.go/database package. Only use this instead of the hive.go package.
package database

import (
	"io/ioutil"
	"os"
	"runtime"
	"sync"

	"github.com/dgraph-io/badger/v2"
	"github.com/dgraph-io/badger/v2/options"
	"github.com/iotaledger/hive.go/database"
	"github.com/iotaledger/hive.go/logger"

	"github.com/iotaledger/goshimmer/plugins/config"
)

var (
	instance       *badger.DB
	once           sync.Once
	ErrKeyNotFound = database.ErrKeyNotFound
)

type (
	Database     = database.Database
	Entry        = database.Entry
	KeyOnlyEntry = database.KeyOnlyEntry
	KeyPrefix    = database.KeyPrefix
	Key          = database.Key
	Value        = database.Value
)

func Get(dbPrefix byte, optionalBadger ...*badger.DB) (Database, error) {
	return database.Get(dbPrefix, optionalBadger...)
}

func GetBadgerInstance() *badger.DB {
	once.Do(func() {
		dbDir := config.Node.GetString(CFG_DIRECTORY)

		var dbDirClear bool
		// check whether the database is new, by checking whether any file exists within
		// the database directory
		fileInfos, err := ioutil.ReadDir(dbDir)
		if err != nil {
			// panic on other errors, for example permission related
			if !os.IsNotExist(err) {
				panic(err)
			}
			dbDirClear = true
		}
		if len(fileInfos) == 0 {
			dbDirClear = true
		}

		opts := badger.DefaultOptions(dbDir)
		opts.Logger = nil
		if runtime.GOOS == "windows" {
			opts = opts.WithTruncate(true)
		}

		opts.SyncWrites = false
		opts.TableLoadingMode = options.MemoryMap
		opts.ValueLogLoadingMode = options.MemoryMap
		opts.CompactL0OnClose = false
		opts.KeepL0InMemory = false
		opts.VerifyValueChecksum = false
		opts.ZSTDCompressionLevel = 1
		opts.Compression = options.None
		opts.MaxCacheSize = 50000000
		opts.EventLogging = false

		db, err := database.CreateDB(dbDir, opts)
		if err != nil {
			// errors should cause a panic to avoid singleton deadlocks
			panic(err)
		}
		instance = db

		// up on the first caller, check whether the version of the database is compatible
		checkDatabaseVersion(dbDirClear)
	})
	return instance
}

func CleanupBadgerInstance(log *logger.Logger) {
	db := GetBadgerInstance()
	log.Info("Running Badger database garbage collection")
	var err error
	for err == nil {
		err = db.RunValueLogGC(0.7)
	}
}
