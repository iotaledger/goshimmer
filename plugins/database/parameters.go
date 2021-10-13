package database

import (
	"time"

	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of configuration parameters used by the storage layer.
type ParametersDefinition struct {
	// Directory defines the directory of the database.
	Directory string `default:"mainnetdb" usage:"path to the database directory"`

	// InMemory defines whether to use an in-memory database.
	InMemory bool `default:"false" usage:"whether the database is only kept in memory and not persisted"`

	// Dirty defines whether to override the database dirty flag.
	Dirty string `default:"false" usage:"set the dirty flag of the database"`

	// ForceCacheTime is a new global cache time in seconds for object storage.
	ForceCacheTime time.Duration `default:"-1s" usage:"interval of time for which objects should remain in memory. Zero time means no caching, negative value means use defaults"`
}

// Parameters contains configuration parameters used by the storage layer.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "database")
}
