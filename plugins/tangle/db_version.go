package tangle

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/parameter"
	"github.com/pkg/errors"
)

var (
	ErrDBVersionIncompatible = errors.New("database version is not compatible. please delete your database folder and restart")
)

const (
	// the database name
	dbVersionDBName = "version"
	// defines which version of the database schema this version of GoShimmer supports.
	// everytime there's a breaking change regarding the stored data, this version flag should be adjusted.
	DBVersion byte = 1
)

var (
	// the key under which the database is stored
	dbVersionKey = []byte{0}
)

func checkDatabaseVersion() {
	var dbDirClear bool
	// only check the version if there's actually a database folder (and it contains files).
	// note that this check only works, if the tangle plugin is the first
	// plugin which initializes a database, as otherwise any other call would automatically
	// create the database folder.
	directory := parameter.NodeConfig.GetString(database.CFG_DIRECTORY)
	fileInfos, err := ioutil.ReadDir(directory)
	if err != nil {
		if !os.IsNotExist(err) {
			panic(err)
		}
		dbDirClear = true
	}
	if len(fileInfos) == 0 {
		dbDirClear = true
	}

	db, err := database.Get(dbVersionDBName)
	if err != nil {
		panic(err)
	}

	if dbDirClear {
		// store db version for the first time in the new database
		if err := db.Set(dbVersionKey, []byte{DBVersion}); err != nil {
			panic(fmt.Sprintf("unable to persist db version number: %s", err.Error()))
		}
		log.Info("storing database version for the first time")
		return
	}

	// db version must be available
	version, err := db.Get(dbVersionKey)
	if err != nil && err != database.ErrKeyNotFound {
		panic(err)
	}
	if version == nil || err != nil {
		panic(errors.Wrapf(ErrDBVersionIncompatible, "no database version was persisted"))
	}
	if version[0] != DBVersion {
		panic(errors.Wrapf(ErrDBVersionIncompatible, "supported version: %d, version of database: %d", DBVersion, version[0]))
	}
	// database version compatible
}
