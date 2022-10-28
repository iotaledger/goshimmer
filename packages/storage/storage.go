package storage

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/storage/headers"
	"github.com/iotaledger/goshimmer/packages/storage/ledger"
	"github.com/iotaledger/goshimmer/packages/storage/tangle"
)

// region Storage //////////////////////////////////////////////////////////////////////////////////////////////////////

type Storage struct {
	*headers.Headers
	Ledger *ledger.Ledger
	Tangle *tangle.Tangle

	database *database.Manager
}

func New(folder string, databaseVersion database.Version) (newStorage *Storage, err error) {
	newStorage = &Storage{
		database: database.NewManager(
			databaseVersion,
			database.WithBaseDir(folder),
			database.WithGranularity(1),
			database.WithDBProvider(database.NewDB),
		),
	}

	if newStorage.Headers, err = headers.New(diskutil.New(folder, true)); err != nil {
		return nil, errors.Errorf("failed to create header storage: %w", err)
	}

	unspentOutputsStorage, unspentOutputIDsStorage, consensusWeightsStorage, err := newStorage.permanentStores()
	if err != nil {
		return nil, errors.Errorf("failed to create ledger storages: %w", err)
	}

	newStorage.Ledger = ledger.New(
		unspentOutputsStorage,
		unspentOutputIDsStorage,
		consensusWeightsStorage,
		newStorage.bucketedStore(LedgerStateDiffsRealm),
	)

	newStorage.Tangle = tangle.New(
		newStorage.bucketedStore(BlockRealm),
		newStorage.bucketedStore(SolidEntryPointsRealm),
		newStorage.bucketedStore(ActivityLogRealm),
	)

	return newStorage, nil
}

func (c *Storage) Shutdown() (err error) {
	c.database.Shutdown()

	return c.Headers.Shutdown()
}

func (c *Storage) permanentStores() (unspentOutputsStorage, unspentOutputIDsStorage, consensusWeightsStorage kvstore.KVStore, err error) {
	if unspentOutputsStorage, err = c.permanentStore(UnspentOutputsRealm); err != nil {
		err = errors.Errorf("failed to create unspent outputs storage: %w", err)
	} else if unspentOutputIDsStorage, err = c.permanentStore(UnspentOutputIDsRealm); err != nil {
		err = errors.Errorf("failed to create unspent output ids storage: %w", err)
	} else if consensusWeightsStorage, err = c.permanentStore(ConsensusWeightsRealm); err != nil {
		err = errors.Errorf("failed to create consensus weights storage: %w", err)
	}

	return unspentOutputsStorage, unspentOutputIDsStorage, consensusWeightsStorage, err
}

func (c *Storage) permanentStore(realm Realm) (storage kvstore.KVStore, err error) {
	if storage, err = c.database.PermanentStorage().WithRealm([]byte{byte(realm)}); err != nil {
		return nil, errors.Errorf("failed to get state storage of type %s: %w", realm, err)
	}

	return storage, nil
}

func (c *Storage) bucketedStore(realm Realm) (bucketedStore func(index epoch.Index) kvstore.KVStore) {
	return newBucketedStorage(c.database, realm).Store
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region bucketedStore ////////////////////////////////////////////////////////////////////////////////////////////////

type bucketedStore struct {
	database *database.Manager
	realm    Realm
}

func newBucketedStorage(database *database.Manager, realm Realm) (newBucketedStore *bucketedStore) {
	return &bucketedStore{
		database: database,
		realm:    realm,
	}
}

func (b *bucketedStore) Store(index epoch.Index) kvstore.KVStore {
	return b.database.Get(index, []byte{byte(b.realm)})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
