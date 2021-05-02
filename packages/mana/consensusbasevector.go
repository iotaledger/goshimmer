package mana

import (
	"bytes"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"golang.org/x/xerrors"
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

// BuildPastBaseVector builds a consensus base mana vector from past events upto time `t`.
// `eventLogs` is expected to be sorted chronologically.
func (c *ConsensusBaseManaVector) BuildPastBaseVector(eventsLog []Event, t time.Time) error {
	if c.vector == nil {
		c.vector = make(map[identity.ID]*ConsensusBaseMana)
	}
	for _, _ev := range eventsLog {
		switch _ev.Type() {
		case EventTypePledge:
			ev := _ev.(*PledgedEvent)
			if ev.Time.After(t) {
				return nil
			}
			if _, exist := c.vector[ev.NodeID]; !exist {
				c.vector[ev.NodeID] = &ConsensusBaseMana{}
			}
			c.vector[ev.NodeID].pledge(txInfoFromPledgeEvent(ev))
		case EventTypeRevoke:
			ev := _ev.(*RevokedEvent)
			if ev.Time.After(t) {
				return nil
			}
			if _, exist := c.vector[ev.NodeID]; !exist {
				c.vector[ev.NodeID] = &ConsensusBaseMana{}
			}
			err := c.vector[ev.NodeID].revoke(ev.Amount, ev.Time)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

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
func (c *ConsensusBaseManaVector) LoadSnapshot(snapshot map[identity.ID]*SnapshotInfo, snapshotTime time.Time) {
	c.Lock()
	defer c.Unlock()

	for nodeID, info := range snapshot {
		c.vector[nodeID] = &ConsensusBaseMana{
			BaseMana1:          info.Value,
			EffectiveBaseMana1: info.Value,
			LastUpdated:        snapshotTime,
		}
		// trigger events
		Events().Pledged.Trigger(&PledgedEvent{
			NodeID:        nodeID,
			Amount:        info.Value,
			Time:          snapshotTime,
			ManaType:      c.Type(),
			TransactionID: info.TxID,
		})
	}
}

// Book books mana for a transaction.
func (c *ConsensusBaseManaVector) Book(txInfo *TxInfo) {
	c.Lock()
	defer c.Unlock()
	// first, revoke mana from previous owners
	for _, inputInfo := range txInfo.InputInfos {
		// which node did the input pledge mana to?
		pledgeNodeID := inputInfo.PledgeID[c.Type()]
		if _, exist := c.vector[pledgeNodeID]; !exist {
			// first time we see this node
			c.vector[pledgeNodeID] = &ConsensusBaseMana{}
		}
		// save old mana
		oldMana := *c.vector[pledgeNodeID]
		// revoke BM1
		err := c.vector[pledgeNodeID].revoke(inputInfo.Amount, txInfo.TimeStamp)
		switch err {
		case ErrBaseManaNegative:
			panic(fmt.Sprintf("Revoking %f base mana 1 from node %s results in negative balance", inputInfo.Amount, pledgeNodeID.String()))
		case ErrEffBaseManaNegative:
			panic(fmt.Sprintf("Revoking (%f) eff base mana 1 from node %s results in negative balance", inputInfo.Amount, pledgeNodeID.String()))
		}
		// trigger events
		Events().Revoked.Trigger(&RevokedEvent{pledgeNodeID, inputInfo.Amount, txInfo.TimeStamp, c.Type(), txInfo.TransactionID, inputInfo.InputID})
		Events().Updated.Trigger(&UpdatedEvent{pledgeNodeID, &oldMana, c.vector[pledgeNodeID], c.Type()})
	}
	// second, pledge mana to new nodes
	pledgeNodeID := txInfo.PledgeID[c.Type()]
	if _, exist := c.vector[pledgeNodeID]; !exist {
		// first time we see this node
		c.vector[pledgeNodeID] = &ConsensusBaseMana{}
	}
	// save it for proper event trigger
	oldMana := *c.vector[pledgeNodeID]
	// actually pledge and update
	pledged := c.vector[pledgeNodeID].pledge(txInfo)

	// trigger events
	Events().Pledged.Trigger(&PledgedEvent{
		NodeID:        pledgeNodeID,
		Amount:        pledged,
		Time:          txInfo.TimeStamp,
		ManaType:      c.Type(),
		TransactionID: txInfo.TransactionID,
	})
	Events().Updated.Trigger(&UpdatedEvent{
		NodeID:   pledgeNodeID,
		OldMana:  &oldMana,
		NewMana:  c.vector[pledgeNodeID],
		ManaType: c.Type(),
	})
}

// Update updates the mana entries for a particular node wrt time.
func (c *ConsensusBaseManaVector) Update(nodeID identity.ID, t time.Time) error {
	c.Lock()
	defer c.Unlock()
	return c.update(nodeID, t)
}

// UpdateAll updates all entries in the base mana vector wrt to `t`.
func (c *ConsensusBaseManaVector) UpdateAll(t time.Time) error {
	c.Lock()
	defer c.Unlock()
	for nodeID := range c.vector {
		if err := c.update(nodeID, t); err != nil {
			return err
		}
	}
	return nil
}

// GetMana returns the Effective Base Mana.
func (c *ConsensusBaseManaVector) GetMana(nodeID identity.ID, optionalUpdateTime ...time.Time) (float64, time.Time, error) {
	c.Lock()
	defer c.Unlock()
	return c.getMana(nodeID, optionalUpdateTime...)
}

// GetManaMap returns mana perception of the node.
func (c *ConsensusBaseManaVector) GetManaMap(optionalUpdateTime ...time.Time) (res NodeMap, t time.Time, err error) {
	c.Lock()
	defer c.Unlock()
	t = time.Now()
	if len(optionalUpdateTime) > 0 {
		t = optionalUpdateTime[0]
	}
	res = make(map[identity.ID]float64)
	for ID := range c.vector {
		var mana float64
		mana, _, err = c.getMana(ID, t)
		if err != nil {
			return nil, t, err
		}
		res[ID] = mana
	}
	return
}

// GetHighestManaNodes return the n highest mana nodes in descending order.
// It also updates the mana values for each node.
// If n is zero, it returns all nodes.
func (c *ConsensusBaseManaVector) GetHighestManaNodes(n uint) (res []Node, t time.Time, err error) {
	err = func() error {
		// don't lock the vector after this func returns
		c.Lock()
		defer c.Unlock()
		t = time.Now()
		for ID := range c.vector {
			var mana float64
			mana, _, err = c.getMana(ID, t)
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
	err = func() error {
		// don't lock the vector after this func returns
		c.Lock()
		defer c.Unlock()
		t = time.Now()
		for ID := range c.vector {
			// skip the empty node ID
			if bytes.Equal(ID[:], emptyNodeID[:]) {
				continue
			}

			var mana float64
			mana, _, err = c.getMana(ID, t)
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
			ManaType:        c.Type(),
			BaseValues:      []float64{bm.BaseValue()},
			EffectiveValues: []float64{bm.EffectiveValue()},
			LastUpdated:     bm.LastUpdated,
			NodeID:          nodeID,
		}
		result = append(result, pbm)
	}
	return result
}

// FromPersistable fills the ConsensusBaseManaVector from persistable mana objects.
func (c *ConsensusBaseManaVector) FromPersistable(p *PersistableBaseMana) (err error) {
	if p.ManaType != ConsensusMana {
		err = xerrors.Errorf("persistable mana object has type %s instead of %s", p.ManaType.String(), ConsensusMana.String())
		return
	}
	if len(p.BaseValues) != 1 {
		err = xerrors.Errorf("persistable mana object has %d base values instead of 1", len(p.BaseValues))
		return
	}
	if len(p.EffectiveValues) != 1 {
		err = xerrors.Errorf("persistable mana object has %d effective values instead of 1", len(p.EffectiveValues))
		return
	}
	c.Lock()
	defer c.Unlock()
	c.vector[p.NodeID] = &ConsensusBaseMana{
		BaseMana1:          p.BaseValues[0],
		EffectiveBaseMana1: p.EffectiveValues[0],
		LastUpdated:        p.LastUpdated,
	}
	return
}

// RemoveZeroNodes removes the zero mana nodes from the vector.
func (c *ConsensusBaseManaVector) RemoveZeroNodes() {
	c.Lock()
	defer c.Unlock()
	for nodeID, baseMana := range c.vector {
		if baseMana.EffectiveValue() < MinEffectiveMana && baseMana.BaseValue() == 0 {
			delete(c.vector, nodeID)
		}
	}
}

var _ BaseManaVector = &ConsensusBaseManaVector{}

//// Region Internal methods ////

// update updates the mana entries for a particular node wrt time. Not concurrency safe.
func (c *ConsensusBaseManaVector) update(nodeID identity.ID, t time.Time) error {
	if _, exist := c.vector[nodeID]; !exist {
		return ErrNodeNotFoundInBaseManaVector
	}
	oldMana := *c.vector[nodeID]
	if err := c.vector[nodeID].update(t); err != nil {
		return err
	}
	Events().Updated.Trigger(&UpdatedEvent{nodeID, &oldMana, c.vector[nodeID], c.Type()})
	return nil
}

// getMana returns the Effective Base Mana 1. Will update base mana by default.
func (c *ConsensusBaseManaVector) getMana(nodeID identity.ID, optionalUpdateTime ...time.Time) (float64, time.Time, error) {
	t := time.Now()
	if _, exist := c.vector[nodeID]; !exist {
		return 0.0, t, ErrNodeNotFoundInBaseManaVector
	}
	if len(optionalUpdateTime) > 0 {
		t = optionalUpdateTime[0]
	}
	_ = c.update(nodeID, t)

	baseMana := c.vector[nodeID]
	return baseMana.EffectiveBaseMana1, t, nil
}
