package storage

import (
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/storage/permanent"
	"github.com/iotaledger/goshimmer/packages/storage/prunable"
)

// Storage is an abstraction around the storage layer of the node.
type Storage struct {
	// Permanent is the section of the storage that is maintained forever (holds the current ledger state).
	*permanent.Permanent

	// Prunable is the section of the storage that is pruned regularly (holds the history of the ledger state).
	*prunable.Prunable

	// Manager is the database manager that manages the underlying database instances.
	database *database.Manager
}

// New creates a new storage instance with the named database version in the given directory.
func New(directory string, version database.Version) (newStorage *Storage) {
	database := database.NewManager(version, database.WithBaseDir(directory), database.WithGranularity(1), database.WithDBProvider(database.NewDB))

	return &Storage{
		Permanent: permanent.New(diskutil.New(directory, true), database),
		Prunable:  prunable.New(database),

		database: database,
	}
}

// Shutdown shuts down the storage.
func (c *Storage) Shutdown() (err error) {
	defer c.database.Shutdown()

	return c.Permanent.Shutdown()
}
