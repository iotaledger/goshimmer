package chainstorage

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/storable"
)

type ChainStorage struct {
	Events *Events

	settings    *settings
	commitments *storable.Slice[commitment.Commitment, *commitment.Commitment]
	database    *database.Manager
}

func NewChainStorage(folder string) (c *ChainStorage) {
	return &ChainStorage{
		settings: &settings{},
	}
}

func (c *ChainStorage) LatestCommittableEpoch() (latestCommittableEpoch epoch.Index) {
	c.settings.RLock()
	defer c.settings.RUnlock()

	return c.settings.LatestCommittableEpoch
}

func (c *ChainStorage) SetLatestCommittableEpoch(latestCommittableEpoch epoch.Index) {
	c.settings.Lock()
	defer c.settings.Unlock()

	c.settings.LatestCommittableEpoch = latestCommittableEpoch

	if err := c.settings.ToFile(); err != nil {
		c.Events.Error.Trigger(errors.Errorf("failed to persist latest committable epoch: %w", err))
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
		c.Events.Error.Trigger(errors.Errorf("failed to set commitment for epoch %d: %w", index, err))
	}
}

func (c *ChainStorage) LedgerstateStorage() (ledgerstateStorage kvstore.KVStore) {
	ledgerstateStorage, err := c.database.PermanentStorage().WithRealm([]byte("ledgerstate"))
	if err != nil {
		c.Events.Error.Trigger(errors.Errorf("failed to get ledgerstate storage: %w", err))
	}

	return ledgerstateStorage
}

func (c *ChainStorage) StateRootStorage() (stateRootStorage kvstore.KVStore) {
	stateRootStorage, err := c.database.PermanentStorage().WithRealm([]byte("stateRoot"))
	if err != nil {
		c.Events.Error.Trigger(errors.Errorf("failed to get state root storage: %w", err))
	}

	return stateRootStorage
}

func (c *ChainStorage) ManaRootStorage() (manaRootStorage kvstore.KVStore) {
	manaRootStorage, err := c.database.PermanentStorage().WithRealm([]byte("manaRoot"))
	if err != nil {
		c.Events.Error.Trigger(errors.Errorf("failed to get mana root storage: %w", err))
	}

	return manaRootStorage
}