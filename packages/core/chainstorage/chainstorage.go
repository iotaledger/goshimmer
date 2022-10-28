package chainstorage

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/types"

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
	State                   *State
	BlockStorage            *BlockStorage
	DiffStorage             *DiffStorage
	SolidEntryPointsStorage *SolidEntryPointsStorage
	ActivityLogStorage      *ActivityLogStorage
	Settings                *Settings
	Commitments             *storable.Slice[commitment.Commitment, *commitment.Commitment]

	database *database.Manager
}

func NewChainStorage(folder string, databaseVersion database.Version) (chainStorage *ChainStorage, err error) {
	chainStorage = &ChainStorage{
		Events: NewEvents(),
	}
	chainStorage.State = NewState(chainStorage)
	chainStorage.BlockStorage = &BlockStorage{chainStorage}
	chainStorage.DiffStorage = &DiffStorage{chainStorage}
	chainStorage.SolidEntryPointsStorage = &SolidEntryPointsStorage{chainStorage}
	chainStorage.ActivityLogStorage = &ActivityLogStorage{chainStorage}

	dbDiskUtil := diskutil.New(folder, true)
	chainStorage.Settings = storable.InitStruct(&Settings{
		LatestCommitment:         commitment.New(0, commitment.ID{}, types.Identifier{}, 0),
		LatestStateMutationEpoch: 0,
		LatestConfirmedEpoch:     0,
	}, dbDiskUtil.Path("settings.bin"))

	if chainStorage.Commitments, err = storable.NewSlice[commitment.Commitment](dbDiskUtil.Path("commitments.bin")); err != nil {
		return nil, errors.Errorf("failed to create commitments database: %w", err)
	}

	chainStorage.database = database.NewManager(databaseVersion, database.WithBaseDir(folder), database.WithGranularity(1), database.WithDBProvider(database.NewDB))

	return chainStorage, nil
}

func (c *ChainStorage) LatestCommitment() (latestCommitment *commitment.Commitment) {
	c.Settings.RLock()
	defer c.Settings.RUnlock()

	return c.Settings.LatestCommitment
}

func (c *ChainStorage) SetLatestCommitment(latestCommitment *commitment.Commitment) {
	c.Settings.Lock()
	defer c.Settings.Unlock()

	c.Settings.LatestCommitment = latestCommitment

	if err := c.Settings.ToFile(); err != nil {
		c.Events.Error.Trigger(errors.Errorf("failed to persist latest commitment: %w", err))
	}
}

func (c *ChainStorage) LatestStateMutationEpoch() (latestStateMutationEpoch epoch.Index) {
	c.Settings.RLock()
	defer c.Settings.RUnlock()

	return c.Settings.LatestStateMutationEpoch
}

func (c *ChainStorage) SetLatestStateMutationEpoch(latestStateMutationEpoch epoch.Index) {
	c.Settings.Lock()
	defer c.Settings.Unlock()

	c.Settings.LatestStateMutationEpoch = latestStateMutationEpoch

	if err := c.Settings.ToFile(); err != nil {
		c.Events.Error.Trigger(errors.Errorf("failed to persist latest state mutation epoch: %w", err))
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
