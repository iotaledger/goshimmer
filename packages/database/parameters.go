package database

import (
	flag "github.com/spf13/pflag"
)

const (
	CFG_DIRECTORY = "database.directory"
)

func init() {
	flag.String(CFG_DIRECTORY, "mainnetdb", "path to the database folder")
}
