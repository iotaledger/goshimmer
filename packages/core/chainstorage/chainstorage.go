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
	ActivityLogStorage      *ActivityLogStorage
	Settings                *Settings
	Commitments             *storable.Slice[commitment.Commitment, *commitment.Commitment]

	database *database.Manager
}

func NewChainStorage(folder string, databaseVersion database.Version) (chainManager *ChainStorage, err error) {
	chainManager = &ChainStorage{
		Events: NewEvents(),
	}
	chainManager.BlockStorage = &BlockStorage{chainManager}
	chainManager.DiffStorage = &DiffStorage{chainManager}
	chainManager.SolidEntryPointsStorage = &SolidEntryPointsStorage{chainManager}
	chainManager.ActivityLogStorage = &ActivityLogStorage{chainManager}

	dbDiskUtil := diskutil.New(folder, true)
	chainManager.Settings = storable.InitStruct(&Settings{
		LatestCommittedEpoch: 0,
		LatestAcceptedEpoch:  0,
		LatestConfirmedEpoch: 0,
	}, dbDiskUtil.Path("settings.bin"))

	if chainManager.Commitments, err = storable.NewSlice[commitment.Commitment](dbDiskUtil.Path("commitments.bin")); err != nil {
		return nil, errors.Errorf("failed to create commitments database: %w", err)
	}

	chainManager.database = database.NewManager(databaseVersion, database.WithBaseDir(folder), database.WithGranularity(1), database.WithDBProvider(database.NewDB))

	return chainManager, nil
}

func (c *ChainStorage) LatestCommittedEpoch() (latestCommittedEpoch epoch.Index) {
	c.Settings.RLock()
	defer c.Settings.RUnlock()

	return c.Settings.LatestCommittedEpoch
}

func (c *ChainStorage) SetLatestCommittedEpoch(latestCommittedEpoch epoch.Index) {
	c.Settings.Lock()
	defer c.Settings.Unlock()

	c.Settings.LatestCommittedEpoch = latestCommittedEpoch

	if err := c.Settings.ToFile(); err != nil {
		c.Events.Error.Trigger(errors.Errorf("failed to persist latest committed epoch: %w", err))
	}
}

func (c *ChainStorage) LatestAcceptedEpoch() (latestAcceptedEpoch epoch.Index) {
	c.Settings.RLock()
	defer c.Settings.RUnlock()

	return c.Settings.LatestAcceptedEpoch
}

func (c *ChainStorage) SetLatestAcceptedEpoch(latestAcceptedEpoch epoch.Index) {
	c.Settings.Lock()
	defer c.Settings.Unlock()

	c.Settings.LatestAcceptedEpoch = latestAcceptedEpoch

	if err := c.Settings.ToFile(); err != nil {
		c.Events.Error.Trigger(errors.Errorf("failed to persist latest accepted epoch: %w", err))
	}
}

func (c *ChainStorage) LatestConfirmedEpoch() (latestConfirmedEpoch epoch.Index) {
	c.Settings.RLock()
	defer c.Settings.RUnlock()

	return c.Settings.LatestConfirmedEpoch
}

func (c *ChainStorage) SetLatestConfirmedEpoch(latestConfirmedEpoch epoch.Index) {
	c.Settings.Lock()
	defer c.Settings.Unlock()

	c.Settings.LatestConfirmedEpoch = latestConfirmedEpoch

	if err := c.Settings.ToFile(); err != nil {
		c.Events.Error.Trigger(errors.Errorf("failed to persist latest confirmed epoch: %w", err))
	}
}

func (c *ChainStorage) Commitment(index epoch.Index) (commitment *commitment.Commitment) {
	commitment, err := c.Commitments.Get(int(index))
	if err != nil {
		c.Events.Error.Trigger(errors.Errorf("failed to get commitment for epoch %d: %w", index, err))
	}

	return
}

func (c *ChainStorage) SetCommitment(index epoch.Index, commitment *commitment.Commitment) {
	if err := c.Commitments.Set(int(index), commitment); err != nil {
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

func (c *ChainStorage) Chain() commitment.ID {
	return c.Settings.Chain
}

func (c *ChainStorage) SetChain(chain commitment.ID) {
	c.Settings.Chain = chain
}

func (c *ChainStorage) Shutdown() {
	c.database.Shutdown()
	c.Commitments.Close()
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
