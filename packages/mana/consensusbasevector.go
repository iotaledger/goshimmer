package mana

import (
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

// BuildPastBaseVector builds a consensus base mana vector from past events upto time `t`
func (c *ConsensusBaseManaVector) BuildPastBaseVector(eventsLog []Event, t time.Time) (int, error) {
	emptyID := identity.ID{}
	if len(c.vector) == 0 {
		c.vector = make(map[identity.ID]*ConsensusBaseMana)
	}
	var i int
	for _, _ev := range eventsLog {
		switch _ev.Type() {
		case EventTypePledge:
			ev := _ev.(*PledgedEvent)
			if ev.Time.After(t) {
				return i, nil
			}
			if ev.NodeID == emptyID {
				continue
			}
			if _, exist := c.vector[ev.NodeID]; !exist {
				c.vector[ev.NodeID] = &ConsensusBaseMana{}
			}
			c.vector[ev.NodeID].pledge(txInfoFromPledgeEvent(ev))
		case EventTypeRevoke:
			ev := _ev.(*RevokedEvent)
			if ev.Time.After(t) {
				return i, nil
			}
			if ev.NodeID == emptyID {
				continue
			}
			if _, exist := c.vector[ev.NodeID]; !exist {
				c.vector[ev.NodeID] = &ConsensusBaseMana{}
			}
			err := c.vector[ev.NodeID].revoke(ev.Amount, ev.Time)
			if err != nil {
				return i, err
			}
		}
		i++
	}
	return i, nil
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

// Book books mana for a transaction.
func (c *ConsensusBaseManaVector) Book(txInfo *TxInfo) {
	c.Lock()
	defer c.Unlock()
	// first, revoke mana from previous owners
	for _, inputInfo := range txInfo.InputInfos {
		// which node did the input pledge mana to?
		pledgeNodeID := inputInfo.PledgeID[c.Type()]
		// can't revoke from genesis
		emptyID := identity.ID{}
		if pledgeNodeID == emptyID {
			continue
		}
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
		Events().Revoked.Trigger(&RevokedEvent{pledgeNodeID, inputInfo.Amount, txInfo.TimeStamp, c.Type(), txInfo.TransactionID})
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
func (c *ConsensusBaseManaVector) GetMana(nodeID identity.ID) (float64, error) {
	c.Lock()
	defer c.Unlock()
	return c.getMana(nodeID)
}

// GetManaMap returns mana perception of the node.
func (c *ConsensusBaseManaVector) GetManaMap() (NodeMap, error) {
	c.Lock()
	defer c.Unlock()
	res := make(map[identity.ID]float64)
	for ID := range c.vector {
		mana, err := c.getMana(ID)
		if err != nil {
			return nil, err
		}
		res[ID] = mana
	}
	return res, nil
}

// GetHighestManaNodes return the n highest mana nodes in descending order.
// It also updates the mana values for each node.
// If n is zero, it returns all nodes.
func (c *ConsensusBaseManaVector) GetHighestManaNodes(n uint) ([]Node, error) {
	var res []Node
	err := func() error {
		// don't lock the vector after this func returns
		c.Lock()
		defer c.Unlock()
		for ID := range c.vector {
			mana, err := c.getMana(ID)
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
		return nil, err
	}

	sort.Slice(res[:], func(i, j int) bool {
		return res[i].Mana > res[j].Mana
	})

	if n == 0 || int(n) >= len(res) {
		return res[:], nil
	}
	return res[:n], nil
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

// getMana returns the Effective Base Mana 1
func (c *ConsensusBaseManaVector) getMana(nodeID identity.ID) (float64, error) {
	if _, exist := c.vector[nodeID]; !exist {
		return 0.0, ErrNodeNotFoundInBaseManaVector
	}
	_ = c.update(nodeID, time.Now())
	baseMana := c.vector[nodeID]
	return baseMana.EffectiveBaseMana1, nil
}
