package database

import (
	flag "github.com/spf13/pflag"
)

const (
	CfgDatabaseDir = "database.directory"
)

func init() {
	flag.String(CfgDatabaseDir, "mainnetdb", "path to the database folder")
}
