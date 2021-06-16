package database

import (
	"github.com/iotaledger/hive.go/configuration"
	flag "github.com/spf13/pflag"
)

const (
	// CfgDatabaseDir defines the directory of the database.
	CfgDatabaseDir = "database.directory"
	// CfgDatabaseInMemory defines whether to use an in-memory database.
	CfgDatabaseInMemory = "database.inMemory"
	// CfgDatabaseDirty defines whether to override the database dirty flag.
	CfgDatabaseDirty = "database.dirty"
)

// Parameters contains configuration parameters used by the storage layer.
var Parameters = struct {
	// ForceCacheTime  is a new global cache time in seconds for object storage.
	ForceCacheTime int `default:"-1" usage:"number of seconds all objects should remain in memory. -1 means use defaults"`
}{}

func init() {
	flag.String(CfgDatabaseDir, "mainnetdb", "path to the database folder")
	flag.Bool(CfgDatabaseInMemory, false, "whether the database is only kept in memory and not persisted")
	flag.String(CfgDatabaseDirty, "", "set the dirty flag of the database")

	configuration.BindParameters(&Parameters, "database")
}
