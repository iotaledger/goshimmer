package storage

import (
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/storage/permanent"
	"github.com/iotaledger/goshimmer/packages/storage/prunable"
)

// region Storage //////////////////////////////////////////////////////////////////////////////////////////////////////

type Storage struct {
	*permanent.Permanent
	*prunable.Prunable

	database *database.Manager
}

func New(folder string, databaseVersion database.Version) (newStorage *Storage) {
	database := database.NewManager(databaseVersion,
		database.WithBaseDir(folder),
		database.WithGranularity(1),
		database.WithDBProvider(database.NewDB),
	)

	return &Storage{
		Permanent: permanent.New(diskutil.New(folder, true), database),
		Prunable:  prunable.New(database),

		database: database,
	}
}

func (c *Storage) Shutdown() (err error) {
	defer c.database.Shutdown()

	return c.Permanent.Shutdown()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
