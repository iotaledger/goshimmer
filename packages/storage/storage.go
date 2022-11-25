package storage

import (
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/storage/permanent"
	"github.com/iotaledger/goshimmer/packages/storage/prunable"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
)

// Storage is an abstraction around the storage layer of the node.
type Storage struct {
	// Permanent is the section of the storage that is maintained forever (holds the current ledger state).
	*permanent.Permanent

	// Prunable is the section of the storage that is pruned regularly (holds the history of the ledger state).
	*prunable.Prunable

	// databaseManager is the database manager.
	databaseManager *database.Manager
}

// New creates a new storage instance with the named database version in the given directory.
func New(directory string, version database.Version, opts ...options.Option[database.Manager]) (newStorage *Storage) {
	databaseManager := database.NewManager(version, append(opts, database.WithBaseDir(directory))...)

	return &Storage{
		Permanent: permanent.New(diskutil.New(directory, true), databaseManager),
		Prunable:  prunable.New(databaseManager),

		databaseManager: databaseManager,
	}
}

// PruneUntilEpoch prunes storage epochs less than and equal to the given index.
func (c *Storage) PruneUntilEpoch(epochIndex epoch.Index) {
	c.databaseManager.PruneUntilEpoch(epochIndex)
}

// Shutdown shuts down the storage.
func (c *Storage) Shutdown() {
	event.Loop.PendingTasksCounter.WaitIsZero()

	defer c.databaseManager.Shutdown()
}
