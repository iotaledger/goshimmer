package prunable

import (
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type Prunable struct {
	Blocks           *Blocks
	SolidEntryPoints *SolidEntryPoints
	ActivityLog      *ActivityLog
	LedgerStateDiffs *LedgerStateDiffs

	database *database.Manager
}

func New(database *database.Manager) (newPrunable *Prunable) {
	newPrunable = &Prunable{
		database: database,
	}
	newPrunable.Blocks = &Blocks{newPrunable.bucketedStore(BlockRealm)}
	newPrunable.SolidEntryPoints = &SolidEntryPoints{newPrunable.bucketedStore(SolidEntryPointsRealm)}
	newPrunable.ActivityLog = &ActivityLog{newPrunable.bucketedStore(ActivityLogRealm)}
	newPrunable.LedgerStateDiffs = &LedgerStateDiffs{BucketedStorage: newPrunable.bucketedStore(LedgerStateDiffsRealm)}

	return newPrunable
}

func (c *Prunable) bucketedStore(realm realm) (bucketedStore func(index epoch.Index) kvstore.KVStore) {
	return newBucketedStorage(c.database, realm).Store
}

// region bucketedStore ////////////////////////////////////////////////////////////////////////////////////////////////

type bucketedStore struct {
	database *database.Manager
	realm    realm
}

func newBucketedStorage(database *database.Manager, realm realm) (newBucketedStore *bucketedStore) {
	return &bucketedStore{
		database: database,
		realm:    realm,
	}
}

func (b *bucketedStore) Store(index epoch.Index) kvstore.KVStore {
	return b.database.Get(index, []byte{byte(b.realm)})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

const (
	BlockRealm realm = iota
	LedgerStateDiffsRealm
	SolidEntryPointsRealm
	ActivityLogRealm
)
