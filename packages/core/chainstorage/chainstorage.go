package chainstorage

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/storable"
)

func init() {
	// SAMPLE API

	// chainStorage.Commitments.Store(*commitment.Commitment)
	// chainStorage.Commitments.Get(index epoch.Index)
	// chainStorage.Commitments.Delete(index epoch.Index)

	// chainStorage.LedgerState.Store(utxo.Output)
	// chainStorage.LedgerState.Get(utxo.OutputID)
	// chainStorage.LedgerState.Delete(utxo.OutputID)

	// chainStorage.ManaStateTree // kvstore.KVStore
	// chainStorage.LedgerStateTree // kvstore.KVStore

	// chainStorage.Blocks.Store(*models.Block)
	// chainStorage.Blocks.Get(models.BlockID)
	// chainStorage.Blocks.Delete(models.BlockID)

	// chainStorage.LedgerDiff(index) // kvstore.KVStore
}

type ChainStorage struct {
	Events                  *Events
	BlockStorage            *BlockStorage
	DiffStorage             *DiffStorage
	SolidEntryPointsStorage *SolidEntryPointsStorage

	settings    *settings
	commitments *storable.Slice[commitment.Commitment, *commitment.Commitment]
	database    *database.Manager
}

func NewChainStorage(folder string, databaseVersion database.Version) (chainManager *ChainStorage, err error) {
	chainManager = &ChainStorage{
		Events: NewEvents(),
	}
	chainManager.BlockStorage = &BlockStorage{chainManager}
	chainManager.DiffStorage = &DiffStorage{chainManager}
	chainManager.SolidEntryPointsStorage = &SolidEntryPointsStorage{chainManager}

	chainManager.settings = storable.InitStruct(&settings{
		LatestCommittedEpoch: 0,
		LatestAcceptedEpoch:  0,
		LatestConfirmedEpoch: 0,
	}, diskutil.New(folder).Path("settings.bin"))

	if chainManager.commitments, err = storable.NewSlice[commitment.Commitment](diskutil.New(folder).Path("commitments.bin")); err != nil {
		return nil, errors.Errorf("failed to create commitments database: %w", err)
	}

	chainManager.database = database.NewManager(databaseVersion, database.WithBaseDir(folder), database.WithGranularity(1), database.WithDBProvider(database.NewDB))

	return chainManager, nil
}

func (c *ChainStorage) LatestCommittedEpoch() (latestCommittedEpoch epoch.Index) {
	c.settings.RLock()
	defer c.settings.RUnlock()

	return c.settings.LatestCommittedEpoch
}

func (c *ChainStorage) SetLatestCommittedEpoch(latestCommittedEpoch epoch.Index) {
	c.settings.Lock()
	defer c.settings.Unlock()

	c.settings.LatestCommittedEpoch = latestCommittedEpoch

	if err := c.settings.ToFile(); err != nil {
		c.Events.Error.Trigger(errors.Errorf("failed to persist latest committed epoch: %w", err))
	}
}

func (c *ChainStorage) LatestAcceptedEpoch() (latestAcceptedEpoch epoch.Index) {
	c.settings.RLock()
	defer c.settings.RUnlock()

	return c.settings.LatestAcceptedEpoch
}

func (c *ChainStorage) SetLatestAcceptedEpoch(latestAcceptedEpoch epoch.Index) {
	c.settings.Lock()
	defer c.settings.Unlock()

	c.settings.LatestAcceptedEpoch = latestAcceptedEpoch

	if err := c.settings.ToFile(); err != nil {
		c.Events.Error.Trigger(errors.Errorf("failed to persist latest accepted epoch: %w", err))
	}
}

func (c *ChainStorage) LatestConfirmedEpoch() (latestConfirmedEpoch epoch.Index) {
	c.settings.RLock()
	defer c.settings.RUnlock()

	return c.settings.LatestConfirmedEpoch
}

func (c *ChainStorage) SetLatestConfirmedEpoch(latestConfirmedEpoch epoch.Index) {
	c.settings.Lock()
	defer c.settings.Unlock()

	c.settings.LatestConfirmedEpoch = latestConfirmedEpoch

	if err := c.settings.ToFile(); err != nil {
		c.Events.Error.Trigger(errors.Errorf("failed to persist latest confirmed epoch: %w", err))
	}
}

func (c *ChainStorage) Commitment(index epoch.Index) (commitment *commitment.Commitment) {
	commitment, err := c.commitments.Get(int(index))
	if err != nil {
		c.Events.Error.Trigger(errors.Errorf("failed to get commitment for epoch %d: %w", index, err))
	}

	return
}

func (c *ChainStorage) SetCommitment(index epoch.Index, commitment *commitment.Commitment) {
	if err := c.commitments.Set(int(index), commitment); err != nil {
		c.Events.Error.Trigger(errors.Errorf("failed to store commitment for epoch %d: %w", index, err))
	}
}

func (c *ChainStorage) LedgerstateStorage() (ledgerstateStorage kvstore.KVStore) {
	return c.permanentStorage(LedgerStateStorageType)
}

func (c *ChainStorage) StateTreeStorage() (stateTreeStorage kvstore.KVStore) {
	return c.permanentStorage(StateTreeStorageType)
}

func (c *ChainStorage) ManaTreeStorage() (manaTreeStorage kvstore.KVStore) {
	return c.permanentStorage(ManaTreeStorageType)
}

func (c *ChainStorage) CommitmentRootsStorage(index epoch.Index) kvstore.KVStore {
	return c.bucketedStorage(index, CommitmentRootsStorageType)
}

func (c *ChainStorage) MutationTreesStorage(index epoch.Index) kvstore.KVStore {
	return c.bucketedStorage(index, MutationTreesStorageType)
}

func (c *ChainStorage) ActivityLogStorage(index epoch.Index) kvstore.KVStore {
	return c.bucketedStorage(index, ActivityLogStorageType)
}

func (c *ChainStorage) Chain() commitment.ID {
	return c.settings.Chain
}

func (c *ChainStorage) SetChain(chain commitment.ID) {
	c.settings.Chain = chain
}

func (c *ChainStorage) Shutdown() {
	c.database.Shutdown()
	c.commitments.Close()
}

func (c *ChainStorage) permanentStorage(storageType Type) (storage kvstore.KVStore) {
	storage, err := c.database.PermanentStorage().WithRealm([]byte{byte(storageType)})
	if err != nil {
		c.Events.Error.Trigger(errors.Errorf("failed to get state storage of type %s: %w", storageType, err))
	}

	return storage
}

func (c *ChainStorage) bucketedStorage(index epoch.Index, storageType Type) (storage kvstore.KVStore) {
	return c.database.Get(index, []byte{byte(storageType)})
}
