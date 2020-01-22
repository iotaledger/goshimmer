package database

import (
	"io/ioutil"
	"os"
	"runtime"
	"sync"

	"github.com/dgraph-io/badger/v2"
	"github.com/iotaledger/goshimmer/packages/parameter"
	"github.com/iotaledger/hive.go/database"
	"github.com/iotaledger/hive.go/logger"
)

var (
	instance *badger.DB
	once     sync.Once
)

func GetGoShimmerBadgerInstance() *badger.DB {
	once.Do(func() {
		dbDir := parameter.NodeConfig.GetString(CFG_DIRECTORY)

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

func CleanupGoShimmerBadgerInstance(log *logger.Logger) {
	db := GetGoShimmerBadgerInstance()
	log.Info("Running Badger database garbage collection")
	var err error
	for err == nil {
		err = db.RunValueLogGC(0.7)
	}
}
