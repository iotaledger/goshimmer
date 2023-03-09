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
	clientList []evilwallet.Client
}

func NewClockSync(slotDuration time.Duration, syncInterval time.Duration, clientList []evilwallet.Client) *ClockSync {
	updateTicker := time.NewTicker(syncInterval)
	return &ClockSync{
		LatestCommittedSlotClock: &SlotClock{slotDuration: slotDuration},

		syncTicker: updateTicker,
		clientList: clientList,
	}
}

func (c *ClockSync) Start() {
	go func() {
		for range c.syncTicker.C {
			c.SynchronizeWith(c.clientList)
		}
	}()
}

func (c *ClockSync) Shutdown() {
	c.syncTicker.Stop()
}

func (c *ClockSync) SynchronizeWith(nodes []evilwallet.Client) {
	slotIndexes := make([]slot.Index, len(nodes))
	for i, clt := range nodes {
		si, err := clt.GetLatestCommittedSlot()
		if err != nil {
			return
		}
		slotIndexes[i] = si
	}
	currentSlot := c.Choose(slotIndexes)
	c.LatestCommittedSlotClock.Update(currentSlot)
}

// Choose returns the slot based on the majority of the nodes.
func (c *ClockSync) Choose(slotIndexes []slot.Index) slot.Index {
	if len(slotIndexes) == 1 {
		return slotIndexes[0]
	}
	count := make(map[slot.Index]int)
	for _, si := range slotIndexes {
		count[si]++
	}
	var maxCount int
	var maxSlot slot.Index
	for si, v := range count {
		if v > maxCount {
			maxCount = v
			maxSlot = si
		}
	}
	return maxSlot
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
