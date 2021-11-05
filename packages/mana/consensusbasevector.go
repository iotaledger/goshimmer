package mana

import (
	"bytes"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/identity"
)

// ConsensusBaseManaVector represents a base mana vector.
type ConsensusBaseManaVector struct {
	vector map[identity.ID]*ConsensusBaseMana
	sync.RWMutex
}

// Type returns the type of this mana vector.
func (c *ConsensusBaseManaVector) Type() Type {
	return ConsensusMana
}

// Size returns the size of this mana vector.
func (c *ConsensusBaseManaVector) Size() int {
	c.RLock()
	defer c.RUnlock()
	return len(c.vector)
}

// Has returns if the given node has mana defined in the vector.
func (c *ConsensusBaseManaVector) Has(nodeID identity.ID) bool {
	c.RLock()
	defer c.RUnlock()
	_, exists := c.vector[nodeID]
	return exists
}

//// BuildPastBaseVector builds a consensus base mana vector from past events upto time `t`.
//// `eventLogs` is expected to be sorted chronologically.
//func (c *ConsensusBaseManaVector) BuildPastBaseVector(eventsLog []Event, t time.Time) error {
//	if c.vector == nil {
//		c.vector = make(map[identity.ID]*ConsensusBaseMana)
//	}
//	for _, _ev := range eventsLog {
//		switch _ev.Type() {
//		case EventTypePledge:
//			ev := _ev.(*PledgedEvent)
//			if ev.Time.After(t) {
//				return nil
//			}
//			if _, exist := c.vector[ev.NodeID]; !exist {
//				c.vector[ev.NodeID] = &ConsensusBaseMana{}
//			}
//			c.vector[ev.NodeID].pledge(txInfoFromPledgeEvent(ev))
//		case EventTypeRevoke:
//			ev := _ev.(*RevokedEvent)
//			if ev.Time.After(t) {
//				return nil
//			}
//			if _, exist := c.vector[ev.NodeID]; !exist {
//				c.vector[ev.NodeID] = &ConsensusBaseMana{}
//			}
//			err := c.vector[ev.NodeID].revoke(ev.Amount, ev.Time)
//			if err != nil {
//				return err
//			}
//		}
//	}
//	return nil
//}

func txInfoFromPledgeEvent(ev *PledgedEvent) *TxInfo {
	return &TxInfo{
		TimeStamp:     ev.Time,
		TransactionID: ev.TransactionID,
		TotalBalance:  ev.Amount,
		PledgeID: map[Type]identity.ID{
			ConsensusMana: ev.NodeID,
		},
		InputInfos: []InputInfo{
			{
				TimeStamp: ev.Time,
				Amount:    ev.Amount,
				PledgeID: map[Type]identity.ID{
					ConsensusMana: ev.NodeID,
				},
			},
		},
	}
}

// LoadSnapshot loads the snapshot.
func (c *ConsensusBaseManaVector) LoadSnapshot(snapshot map[identity.ID]SnapshotNode) {
	c.Lock()
	defer c.Unlock()

	for nodeID, records := range snapshot {
		var value float64
		for _, record := range records.SortedTxSnapshot {
			value += record.Value

			// trigger event
			Events().Pledged.Trigger(&PledgedEvent{
				NodeID:        nodeID,
				Amount:        record.Value,
				Time:          record.Timestamp,
				ManaType:      c.Type(),
				TransactionID: record.TxID,
			})
		}

		c.vector[nodeID] = &ConsensusBaseMana{
			BaseMana1: value,
		}
	}
}

// Book books mana for a transaction.
func (c *ConsensusBaseManaVector) Book(txInfo *TxInfo) {
	// gather events to be triggered once the lock is lifted
	var revokeEvents []*RevokedEvent
	var pledgeEvents []*PledgedEvent
	var updateEvents []*UpdatedEvent
	// only lock mana vector while we are working with it
	func() {
		c.Lock()
		defer c.Unlock()
		// first, revoke mana from previous owners
		for _, inputInfo := range txInfo.InputInfos {
			// which node did the input pledge mana to?
			oldPledgeNodeID := inputInfo.PledgeID[c.Type()]
			if _, exist := c.vector[oldPledgeNodeID]; !exist {
				// first time we see this node
				c.vector[oldPledgeNodeID] = &ConsensusBaseMana{}
			}
			// save old mana
			oldMana := *c.vector[oldPledgeNodeID]
			// revoke BM1
			err := c.vector[oldPledgeNodeID].revoke(inputInfo.Amount)
			if errors.Is(err, ErrBaseManaNegative) {
				panic(fmt.Sprintf("Revoking %f base mana 1 from node %s results in negative balance", inputInfo.Amount, oldPledgeNodeID.String()))
			}
			// save events for later triggering
			revokeEvents = append(revokeEvents, &RevokedEvent{oldPledgeNodeID, inputInfo.Amount, txInfo.TimeStamp, c.Type(), txInfo.TransactionID, inputInfo.InputID})
			updateEvents = append(updateEvents, &UpdatedEvent{oldPledgeNodeID, &oldMana, c.vector[oldPledgeNodeID], c.Type()})
		}
		// second, pledge mana to new nodes
		newPledgeNodeID := txInfo.PledgeID[c.Type()]
		if _, exist := c.vector[newPledgeNodeID]; !exist {
			// first time we see this node
			c.vector[newPledgeNodeID] = &ConsensusBaseMana{}
		}
		// save it for proper event trigger
		oldMana := *c.vector[newPledgeNodeID]
		// actually pledge and update
		pledged := c.vector[newPledgeNodeID].pledge(txInfo)
		pledgeEvents = append(pledgeEvents, &PledgedEvent{
			NodeID:        newPledgeNodeID,
			Amount:        pledged,
			Time:          txInfo.TimeStamp,
			ManaType:      c.Type(),
			TransactionID: txInfo.TransactionID,
		})
		updateEvents = append(updateEvents, &UpdatedEvent{
			NodeID:   newPledgeNodeID,
			OldMana:  &oldMana,
			NewMana:  c.vector[newPledgeNodeID],
			ManaType: c.Type(),
		})
	}()

	// trigger the events once we released the lock on the mana vector
	for _, ev := range revokeEvents {
		Events().Revoked.Trigger(ev)
	}
	for _, ev := range pledgeEvents {
		Events().Pledged.Trigger(ev)
	}
	for _, ev := range updateEvents {
		Events().Updated.Trigger(ev)
	}
}

// Update updates the mana entries for a particular node wrt time.
func (c *ConsensusBaseManaVector) Update(nodeID identity.ID, t time.Time) error {
	panic("not implemented")
}

// UpdateAll updates all entries in the base mana vector wrt to `t`.
func (c *ConsensusBaseManaVector) UpdateAll(t time.Time) error {
	panic("not implemented")
}

// GetMana returns the Effective Base Mana.
func (c *ConsensusBaseManaVector) GetMana(nodeID identity.ID, optionalUpdateTime ...time.Time) (float64, time.Time, error) {
	c.Lock()
	defer c.Unlock()
	mana, err := c.getMana(nodeID)
	return mana, time.Now(), err
}

// GetManaMap returns mana perception of the node.
func (c *ConsensusBaseManaVector) GetManaMap(optionalUpdateTime ...time.Time) (res NodeMap, t time.Time, err error) {
	c.Lock()
	defer c.Unlock()
	t = time.Now()
	res = make(map[identity.ID]float64, len(c.vector))
	for ID, val := range c.vector {
		res[ID] = val.BaseValue()
	}
	return
}

// GetHighestManaNodes return the n highest mana nodes in descending order.
// It also updates the mana values for each node.
// If n is zero, it returns all nodes.
func (c *ConsensusBaseManaVector) GetHighestManaNodes(n uint) (res []Node, t time.Time, err error) {
	t = time.Now()
	err = func() error {
		// don't lock the vector after this func returns
		c.Lock()
		defer c.Unlock()
		for ID := range c.vector {
			var mana float64
			mana, err = c.getMana(ID)
			if err != nil {
				return err
			}
			res = append(res, Node{
				ID:   ID,
				Mana: mana,
			})
		}
		return nil
	}()
	if err != nil {
		return nil, t, err
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].Mana > res[j].Mana
	})

	if n == 0 || int(n) >= len(res) {
		return
	}
	res = res[:n]
	return
}

// GetHighestManaNodesFraction returns the highest mana that own 'p' percent of total mana.
// It also updates the mana values for each node.
// If p is zero or greater than one, it returns all nodes.
func (c *ConsensusBaseManaVector) GetHighestManaNodesFraction(p float64) (res []Node, t time.Time, err error) {
	emptyNodeID := identity.ID{}
	totalMana := 0.0
	t = time.Now()
	err = func() error {
		// don't lock the vector after this func returns
		c.Lock()
		defer c.Unlock()
		for ID := range c.vector {
			// skip the empty node ID
			if bytes.Equal(ID[:], emptyNodeID[:]) {
				continue
			}

			var mana float64
			mana, err = c.getMana(ID)
			if err != nil {
				return err
			}
			res = append(res, Node{
				ID:   ID,
				Mana: mana,
			})
			totalMana += mana
		}
		return nil
	}()
	if err != nil {
		return nil, t, err
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].Mana > res[j].Mana
	})

	// how much mana is p percent of total mana
	manaThreshold := p * totalMana
	// include nodes as long as their counted mana is less than the threshold
	manaCounted := 0.0
	var n uint
	for n = 0; int(n) < len(res) && manaCounted < manaThreshold; n++ {
		manaCounted += res[n].Mana
	}

	if n == 0 || int(n) >= len(res) {
		return
	}
	res = res[:n]
	return res, t, err
}

// SetMana sets the base mana for a node.
func (c *ConsensusBaseManaVector) SetMana(nodeID identity.ID, bm BaseMana) {
	c.Lock()
	defer c.Unlock()
	c.vector[nodeID] = bm.(*ConsensusBaseMana)
}

// ForEach iterates over the vector and calls the provided callback.
func (c *ConsensusBaseManaVector) ForEach(callback func(ID identity.ID, bm BaseMana) bool) {
	// lock to be on the safe side, although callback might just read
	c.Lock()
	defer c.Unlock()
	for nodeID, baseMana := range c.vector {
		if !callback(nodeID, baseMana) {
			return
		}
	}
}

// ToPersistables converts the baseManaVector to a list of persistable mana objects.
func (c *ConsensusBaseManaVector) ToPersistables() []*PersistableBaseMana {
	c.RLock()
	defer c.RUnlock()
	var result []*PersistableBaseMana
	for nodeID, bm := range c.vector {
		pbm := &PersistableBaseMana{
			ManaType:   c.Type(),
			BaseValues: []float64{bm.BaseValue()},
			NodeID:     nodeID,
		}
		result = append(result, pbm)
	}
	return result
}

// FromPersistable fills the ConsensusBaseManaVector from persistable mana objects.
func (c *ConsensusBaseManaVector) FromPersistable(p *PersistableBaseMana) (err error) {
	if p.ManaType != ConsensusMana {
		err = errors.Errorf("persistable mana object has type %s instead of %s", p.ManaType.String(), ConsensusMana.String())
		return
	}
	if len(p.BaseValues) != 1 {
		err = errors.Errorf("persistable mana object has %d base values instead of 1", len(p.BaseValues))
		return
	}
	c.Lock()
	defer c.Unlock()
	c.vector[p.NodeID] = &ConsensusBaseMana{
		BaseMana1: p.BaseValues[0],
	}
	return
}

// RemoveZeroNodes removes the zero mana nodes from the vector.
func (c *ConsensusBaseManaVector) RemoveZeroNodes() {
	c.Lock()
	defer c.Unlock()
	for nodeID, baseMana := range c.vector {
		if baseMana.BaseValue() == 0 {
			delete(c.vector, nodeID)
		}
	}
}

var _ BaseManaVector = &ConsensusBaseManaVector{}

//// Region Internal methods ////

// getMana returns the consensus mana.
func (c *ConsensusBaseManaVector) getMana(nodeID identity.ID) (float64, error) {
	if _, exist := c.vector[nodeID]; !exist {
		return 0.0, ErrNodeNotFoundInBaseManaVector
	}

	baseMana := c.vector[nodeID]
	return baseMana.BaseValue(), nil
}
