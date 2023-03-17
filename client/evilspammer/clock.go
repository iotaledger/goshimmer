package evilspammer

import (
	"time"

	"github.com/iotaledger/goshimmer/client/evilwallet"
	"github.com/iotaledger/hive.go/core/slot"
)

// region ClockSync ///////////////////////////////////////////////////////////////////////////////////////////////

// ClockSync is used to synchronize with connected nodes.
type ClockSync struct {
	LatestCommittedSlotClock *SlotClock

	syncTicker *time.Ticker
	clt        evilwallet.Client
}

func NewClockSync(slotDuration time.Duration, syncInterval time.Duration, clientList evilwallet.Client) *ClockSync {
	updateTicker := time.NewTicker(syncInterval)
	return &ClockSync{
		LatestCommittedSlotClock: &SlotClock{slotDuration: slotDuration},

		syncTicker: updateTicker,
		clt:        clientList,
	}
}

// Start starts the clock synchronization in the background after the first sync is done..
func (c *ClockSync) Start() {
	c.Synchronize()
	go func() {
		for range c.syncTicker.C {
			c.Synchronize()
		}
	}()
}

func (c *ClockSync) Shutdown() {
	c.syncTicker.Stop()
}

func (c *ClockSync) Synchronize() {
	si, err := c.clt.GetLatestCommittedSlot()
	if err != nil {
		return
	}
	c.LatestCommittedSlotClock.Update(si)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////

// region SlotClock ///////////////////////////////////////////////////////////////////////////////////////////////

type SlotClock struct {
	lastUpdated time.Time
	updatedSlot slot.Index

	slotDuration time.Duration
}

func (c *SlotClock) Update(value slot.Index) {
	c.lastUpdated = time.Now()
	c.updatedSlot = value
}

func (c *SlotClock) Get() slot.Index {
	return c.updatedSlot
}

func (c *SlotClock) GetRelative() slot.Index {
	return c.updatedSlot + slot.Index(time.Since(c.lastUpdated)/c.slotDuration)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////
