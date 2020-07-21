package database

import (
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

func init() {
	flag.String(CfgDatabaseDir, "mainnetdb", "path to the database folder")
	flag.Bool(CfgDatabaseInMemory, false, "whether the database is only kept in memory and not persisted")
	flag.String(CfgDatabaseDirty, "", "set the dirty flag of the database")
}
