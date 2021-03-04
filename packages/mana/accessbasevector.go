package mana

import (
	"sort"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"golang.org/x/xerrors"
)

// AccessBaseManaVector represents a base mana vector.
type AccessBaseManaVector struct {
	vector map[identity.ID]*AccessBaseMana
	sync.RWMutex
}

// Type returns the type of this mana vector.
func (a *AccessBaseManaVector) Type() Type {
	return AccessMana
}

// Size returns the size of this mana vector.
func (a *AccessBaseManaVector) Size() int {
	a.RLock()
	defer a.RUnlock()
	return len(a.vector)
}

// Has returns if the given node has mana defined in the vector.
func (a *AccessBaseManaVector) Has(nodeID identity.ID) bool {
	a.RLock()
	defer a.RUnlock()
	_, exists := a.vector[nodeID]
	return exists
}

// Book books mana for a transaction.
func (a *AccessBaseManaVector) Book(txInfo *TxInfo) {
	a.Lock()
	defer a.Unlock()
	pledgeNodeID := txInfo.PledgeID[a.Type()]
	if _, exist := a.vector[pledgeNodeID]; !exist {
		// first time we see this node
		a.vector[pledgeNodeID] = &AccessBaseMana{}
	}
	// save it for proper event trigger
	oldMana := *a.vector[pledgeNodeID]
	// actually pledge and update
	pledged := a.vector[pledgeNodeID].pledge(txInfo)

	// trigger events
	Events().Pledged.Trigger(&PledgedEvent{
		NodeID:        pledgeNodeID,
		Amount:        pledged,
		Time:          txInfo.TimeStamp,
		ManaType:      a.Type(),
		TransactionID: txInfo.TransactionID,
	})
	Events().Updated.Trigger(&UpdatedEvent{
		NodeID:   pledgeNodeID,
		OldMana:  &oldMana,
		NewMana:  a.vector[pledgeNodeID],
		ManaType: a.Type(),
	})
}

// Update updates the mana entries for a particular node wrt time.
func (a *AccessBaseManaVector) Update(nodeID identity.ID, t time.Time) error {
	a.Lock()
	defer a.Unlock()
	return a.update(nodeID, t)
}

// UpdateAll updates all entries in the base mana vector wrt to `t`.
func (a *AccessBaseManaVector) UpdateAll(t time.Time) error {
	a.Lock()
	defer a.Unlock()
	for nodeID := range a.vector {
		if err := a.update(nodeID, t); err != nil {
			return err
		}
	}
	return nil
}

// GetMana returns Effective Base Mana 2.
func (a *AccessBaseManaVector) GetMana(nodeID identity.ID, optionalUpdateTime ...time.Time) (float64, time.Time, error) {
	a.Lock()
	defer a.Unlock()
	return a.getMana(nodeID, optionalUpdateTime...)
}

// GetManaMap returns mana perception of the node..
func (a *AccessBaseManaVector) GetManaMap(optionalUpdateTime ...time.Time) (res NodeMap, t time.Time, err error) {
	a.Lock()
	defer a.Unlock()
	t = time.Now()
	if len(optionalUpdateTime) > 0 {
		t = optionalUpdateTime[0]
	}
	res = make(map[identity.ID]float64)
	for ID := range a.vector {
		var mana float64
		mana, _, err = a.getMana(ID, t)
		if err != nil {
			return
		}
		res[ID] = mana
	}
	return
}

// GetHighestManaNodes returns the n highest mana nodes in descending order.
// It also updates the mana values for each node.
// If n is zero, it returns all nodes.
func (a *AccessBaseManaVector) GetHighestManaNodes(n uint) (res []Node, t time.Time, err error) {
	err = func() error {
		// don't lock the vector after this func returns
		a.Lock()
		defer a.Unlock()
		t = time.Now()
		for ID := range a.vector {
			var mana float64
			mana, _, err = a.getMana(ID, t)
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

	sort.Slice(res[:], func(i, j int) bool {
		return res[i].Mana > res[j].Mana
	})

	if n == 0 || int(n) >= len(res) {
		return
	}
	res = res[:n]
	return
}

// SetMana sets the base mana for a node.
func (a *AccessBaseManaVector) SetMana(nodeID identity.ID, bm BaseMana) {
	a.Lock()
	defer a.Unlock()
	a.vector[nodeID] = bm.(*AccessBaseMana)
}

// ForEach iterates over the vector and calls the provided callback.
func (a *AccessBaseManaVector) ForEach(callback func(ID identity.ID, bm BaseMana) bool) {
	// lock to be on the safe side, although callback might just read
	a.Lock()
	defer a.Unlock()
	for nodeID, baseMana := range a.vector {
		if !callback(nodeID, baseMana) {
			return
		}
	}
}

// ToPersistables converts the AccessBaseManaVector to a list of persistable mana objects.
func (a *AccessBaseManaVector) ToPersistables() []*PersistableBaseMana {
	a.RLock()
	defer a.RUnlock()
	var result []*PersistableBaseMana
	for nodeID, bm := range a.vector {
		pbm := &PersistableBaseMana{
			ManaType:        a.Type(),
			BaseValues:      []float64{bm.BaseValue()},
			EffectiveValues: []float64{bm.EffectiveValue()},
			LastUpdated:     bm.LastUpdated,
			NodeID:          nodeID,
		}
		result = append(result, pbm)
	}
	return result
}

// FromPersistable fills the AccessBaseManaVector from persistable mana objects.
func (a *AccessBaseManaVector) FromPersistable(p *PersistableBaseMana) (err error) {
	if p.ManaType != AccessMana {
		err = xerrors.Errorf("persistable mana object has type %s instead of %s", p.ManaType.String(), AccessMana.String())
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
	a.Lock()
	defer a.Unlock()
	a.vector[p.NodeID] = &AccessBaseMana{
		BaseMana2:          p.BaseValues[0],
		EffectiveBaseMana2: p.EffectiveValues[0],
		LastUpdated:        p.LastUpdated,
	}
	return
}

var _ BaseManaVector = &AccessBaseManaVector{}

//// Region Internal methods ////

// update updates the mana entries for a particular node wrt time. Not concurrency safe.
func (a *AccessBaseManaVector) update(nodeID identity.ID, t time.Time) error {
	if _, exist := a.vector[nodeID]; !exist {
		return ErrNodeNotFoundInBaseManaVector
	}
	oldMana := *a.vector[nodeID]
	if err := a.vector[nodeID].update(t); err != nil {
		return err
	}
	Events().Updated.Trigger(&UpdatedEvent{nodeID, &oldMana, a.vector[nodeID], a.Type()})
	return nil
}

// getMana returns the current effective mana value. Not concurrency safe.
func (a *AccessBaseManaVector) getMana(nodeID identity.ID, optionalUpdateTime ...time.Time) (float64, time.Time, error) {
	t := time.Now()
	if _, exist := a.vector[nodeID]; !exist {
		return 0.0, t, ErrNodeNotFoundInBaseManaVector
	}
	if len(optionalUpdateTime) > 0 {
		t = optionalUpdateTime[0]
	}
	_ = a.update(nodeID, t)

	baseMana := a.vector[nodeID]
	return baseMana.EffectiveValue(), t, nil
}
