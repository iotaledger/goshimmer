package mana

import (
	"sort"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/model"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/tangle"
)

// AccessBaseManaVector represents a base mana vector.
type AccessBaseManaVector struct {
	model.Mutable[AccessBaseManaVector, *AccessBaseManaVector, accessBaseManaVectorModel] `serix:"0"`
}

type accessBaseManaVectorModel struct {
	Vector map[identity.ID]*AccessBaseMana `serix:"0"`
}

// Vector returns the AccessBaseMana vector.
func (a *AccessBaseManaVector) Vector() map[identity.ID]*AccessBaseMana {
	return a.M.Vector
}

// Type returns the type of this mana vector.
func (a *AccessBaseManaVector) Type() Type {
	return AccessMana
}

// Size returns the size of this mana vector.
func (a *AccessBaseManaVector) Size() int {
	a.RLock()
	defer a.RUnlock()
	return len(a.M.Vector)
}

// Has returns if the given node has mana defined in the vector.
func (a *AccessBaseManaVector) Has(nodeID identity.ID) bool {
	a.RLock()
	defer a.RUnlock()
	_, exists := a.M.Vector[nodeID]
	return exists
}

// LoadSnapshot loads the initial mana state into the base mana vector.
func (a *AccessBaseManaVector) LoadSnapshot(snapshot map[identity.ID]*SnapshotNode) {
	a.Lock()
	defer a.Unlock()

	// pledging aMana to nodes present in the snapshot as if all was pledged at Timestamp
	for nodeID, record := range snapshot {
		a.M.Vector[nodeID] = NewAccessBaseMana(record.AccessMana.Value, record.AccessMana.Value, record.AccessMana.Timestamp)

		// trigger events
		Events.Pledged.Trigger(&PledgedEvent{
			NodeID:   nodeID,
			Amount:   record.AccessMana.Value,
			Time:     record.AccessMana.Timestamp,
			ManaType: a.Type(),
		})
	}
}

// Book books mana for a transaction.
func (a *AccessBaseManaVector) Book(txInfo *TxInfo) {
	var pledgeEvent *PledgedEvent
	var updateEvent *UpdatedEvent
	func() {
		a.Lock()
		defer a.Unlock()
		pledgeNodeID := txInfo.PledgeID[a.Type()]
		if _, exist := a.M.Vector[pledgeNodeID]; !exist {
			// first time we see this node
			a.M.Vector[pledgeNodeID] = NewAccessBaseMana(0, 0, time.Time{})
		}
		// save it for proper event trigger
		oldMana := *a.M.Vector[pledgeNodeID]
		// actually pledge and update
		pledged := a.M.Vector[pledgeNodeID].pledge(txInfo)
		pledgeEvent = &PledgedEvent{
			NodeID:        pledgeNodeID,
			Amount:        pledged,
			Time:          txInfo.TimeStamp,
			ManaType:      a.Type(),
			TransactionID: txInfo.TransactionID,
		}
		updateEvent = &UpdatedEvent{
			NodeID:   pledgeNodeID,
			OldMana:  &oldMana,
			NewMana:  a.M.Vector[pledgeNodeID],
			ManaType: a.Type(),
		}
	}()
	// trigger events
	Events.Pledged.Trigger(pledgeEvent)
	Events.Updated.Trigger(updateEvent)
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
	for nodeID := range a.M.Vector {
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
	for ID := range a.M.Vector {
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
		for ID := range a.M.Vector {
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
func (a *AccessBaseManaVector) GetHighestManaNodesFraction(p float64) (res []Node, t time.Time, err error) {
	totalMana := 0.0
	err = func() error {
		// don't lock the vector after this func returns
		a.Lock()
		defer a.Unlock()
		t = time.Now()
		for ID := range a.M.Vector {
			var mana float64
			mana, _, err = a.getMana(ID, t)
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
func (a *AccessBaseManaVector) SetMana(nodeID identity.ID, bm BaseMana) {
	a.Lock()
	defer a.Unlock()
	a.M.Vector[nodeID] = bm.(*AccessBaseMana)
}

// ForEach iterates over the vector and calls the provided callback.
func (a *AccessBaseManaVector) ForEach(callback func(ID identity.ID, bm BaseMana) bool) {
	// lock to be on the safe side, although callback might just read
	a.Lock()
	defer a.Unlock()
	for nodeID, baseMana := range a.M.Vector {
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
	for nodeID, bm := range a.M.Vector {
		pbm := NewPersistableBaseMana(nodeID, a.Type(), []float64{bm.EffectiveValue()}, []float64{bm.EffectiveValue()}, bm.LastUpdate())
		result = append(result, pbm)
	}
	return result
}

// FromPersistable fills the AccessBaseManaVector from persistable mana objects.
func (a *AccessBaseManaVector) FromPersistable(p *PersistableBaseMana) (err error) {
	if p.ManaType() != AccessMana {
		err = errors.Errorf("persistable mana object has type %s instead of %s", p.ManaType().String(), AccessMana.String())
		return
	}
	if len(p.BaseValues()) != 1 {
		err = errors.Errorf("persistable mana object has %d base values instead of 1", len(p.BaseValues()))
		return
	}
	if len(p.EffectiveValues()) != 1 {
		err = errors.Errorf("persistable mana object has %d effective values instead of 1", len(p.EffectiveValues()))
		return
	}
	a.Lock()
	defer a.Unlock()
	a.M.Vector[p.NodeID()] = NewAccessBaseMana(p.BaseValues()[0], p.EffectiveValues()[0], p.LastUpdated())
	return
}

// RemoveZeroNodes removes the zero mana nodes from the vector.
func (a *AccessBaseManaVector) RemoveZeroNodes() {
	a.Lock()
	defer a.Unlock()
	for nodeID, baseMana := range a.M.Vector {
		if baseMana.EffectiveValue() < MinMana {
			delete(a.M.Vector, nodeID)
		}
	}
}

var _ BaseManaVector = &AccessBaseManaVector{}

// // Region Internal methods ////

// update updates the mana entries for a particular node wrt time. Not concurrency safe.
func (a *AccessBaseManaVector) update(nodeID identity.ID, t time.Time) error {
	if _, exist := a.M.Vector[nodeID]; !exist {
		return ErrNodeNotFoundInBaseManaVector
	}
	oldMana := *a.M.Vector[nodeID]
	if err := a.M.Vector[nodeID].update(t); err != nil {
		return err
	}
	Events.Updated.Trigger(&UpdatedEvent{nodeID, &oldMana, a.M.Vector[nodeID], a.Type()})
	return nil
}

// getMana returns the current effective mana value. Not concurrency safe.
func (a *AccessBaseManaVector) getMana(nodeID identity.ID, optionalUpdateTime ...time.Time) (float64, time.Time, error) {
	t := time.Now()
	if _, exist := a.M.Vector[nodeID]; !exist {
		return 0, t, nil
	}
	if len(optionalUpdateTime) > 0 {
		t = optionalUpdateTime[0]
	}
	_ = a.update(nodeID, t)

	baseMana := a.M.Vector[nodeID]
	effectiveValue := baseMana.EffectiveValue()
	if effectiveValue < tangle.MinMana {
		effectiveValue = 0
	}
	return effectiveValue, t, nil
}
