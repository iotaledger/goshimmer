package database

import (
	"time"

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
	// CacheTimeProvider  is a new global cache time in seconds for object storage.
	ForceCacheTime time.Duration `default:"-1s" usage:"interval of time for which objects should remain in memory. Zero time means no caching, negative value means use defaults"`
}{}

func init() {
	flag.String(CfgDatabaseDir, "mainnetdb", "path to the database folder")
	flag.Bool(CfgDatabaseInMemory, false, "whether the database is only kept in memory and not persisted")
	flag.String(CfgDatabaseDirty, "", "set the dirty flag of the database")

	configuration.BindParameters(&Parameters, "database")
}
