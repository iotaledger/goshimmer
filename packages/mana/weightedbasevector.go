package mana

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"golang.org/x/xerrors"
)

// WeightedBaseManaVector represents a base mana vector.
type WeightedBaseManaVector struct {
	vector map[identity.ID]*WeightedBaseMana
	weight float64
	target Type
	sync.RWMutex
}

// Type returns the type of this mana vector.
func (w *WeightedBaseManaVector) Type() Type {
	return WeightedMana
}

// Target returns the target type of this mana vector, namely if it takes into account access or consensus pledges.
func (w *WeightedBaseManaVector) Target() Type {
	return w.target
}

// Size returns the size of this mana vector.
func (w *WeightedBaseManaVector) Size() int {
	w.RLock()
	defer w.RUnlock()
	return len(w.vector)
}

// SetWeight sets the weight for the whole vector.
func (w *WeightedBaseManaVector) SetWeight(weight float64) error {
	if weight < OnlyMana2 || weight > OnlyMana1 {
		return xerrors.Errorf("error while setting weight to %f: %w", weight, ErrInvalidWeightParameter)
	}
	w.weight = weight
	for _, bm := range w.vector {
		_ = bm.SetWeight(w.weight)
	}
	return nil
}

// Has returns if the given node has mana defined in the vector.
func (w *WeightedBaseManaVector) Has(nodeID identity.ID) bool {
	w.RLock()
	defer w.RUnlock()
	_, exists := w.vector[nodeID]
	return exists
}

// LoadSnapshot loads the initial mana state into the base mana vector.
func (w *WeightedBaseManaVector) LoadSnapshot(snapshot map[identity.ID]*SnapshotInfo, snapshotTime time.Time) {
	w.Lock()
	defer w.Unlock()

	for nodeID, info := range snapshot {
		w.vector[nodeID] = NewWeightedMana(w.weight)
		cBase := &ConsensusBaseMana{
			BaseMana1:          info.Value,
			EffectiveBaseMana1: info.Value,
			LastUpdated:        snapshotTime,
		}
		w.SetMana1(nodeID, cBase)
		Events().Pledged.Trigger(&PledgedEvent{
			NodeID:        nodeID,
			Amount:        info.Value,
			Time:          snapshotTime,
			ManaType:      w.Type(),
			TransactionID: info.TxID,
		})
	}
}

// Book books mana for a transaction.
func (w *WeightedBaseManaVector) Book(txInfo *TxInfo) {
	w.Lock()
	defer w.Unlock()
	// first, revoke mana from previous owners
	for _, inputInfo := range txInfo.InputInfos {
		// which node did the input pledge mana to?
		pledgeNodeID := inputInfo.PledgeID[w.target]
		// can't revoke from genesis
		//emptyID := identity.ID{}
		//if pledgeNodeID == emptyID {
		//	continue
		//}
		if _, exist := w.vector[pledgeNodeID]; !exist {
			// first time we see this node
			w.vector[pledgeNodeID] = NewWeightedMana(w.weight)
		}
		// save old mana
		oldMana := *w.vector[pledgeNodeID]
		// revoke BM1
		err := w.vector[pledgeNodeID].revoke(inputInfo.Amount, txInfo.TimeStamp)
		switch err {
		case ErrBaseManaNegative:
			panic(fmt.Sprintf("Revoking %f base mana 1 from node %s results in negative balance", inputInfo.Amount, pledgeNodeID.String()))
		case ErrEffBaseManaNegative:
			panic(fmt.Sprintf("Revoking (%f) eff base mana 1 from node %s results in negative balance", inputInfo.Amount, pledgeNodeID.String()))
		}
		// trigger events
		Events().Revoked.Trigger(&RevokedEvent{pledgeNodeID, inputInfo.Amount, txInfo.TimeStamp, w.Type(), txInfo.TransactionID, inputInfo.InputID})
		Events().Updated.Trigger(&UpdatedEvent{pledgeNodeID, &oldMana, w.vector[pledgeNodeID], w.Type()})
	}
	pledgeNodeID := txInfo.PledgeID[w.Target()]
	if _, exist := w.vector[pledgeNodeID]; !exist {
		// first time we see this node
		w.vector[pledgeNodeID] = NewWeightedMana(w.weight)
	}
	// save it for proper event trigger
	oldMana := *w.vector[pledgeNodeID]
	// actually pledge and update
	pledged := w.vector[pledgeNodeID].pledge(txInfo)

	// trigger events
	Events().Pledged.Trigger(&PledgedEvent{
		NodeID:        pledgeNodeID,
		Amount:        pledged,
		Time:          txInfo.TimeStamp,
		ManaType:      w.Type(),
		TransactionID: txInfo.TransactionID,
	})
	Events().Updated.Trigger(&UpdatedEvent{
		NodeID:   pledgeNodeID,
		OldMana:  &oldMana,
		NewMana:  w.vector[pledgeNodeID],
		ManaType: w.Type(),
	})
}

// Update updates the mana entries for a particular node wrt time.
func (w *WeightedBaseManaVector) Update(nodeID identity.ID, t time.Time) error {
	w.Lock()
	defer w.Unlock()
	return w.update(nodeID, t)
}

// UpdateAll updates all entries in the base mana vector wrt to `t`.
func (w *WeightedBaseManaVector) UpdateAll(t time.Time) error {
	w.Lock()
	defer w.Unlock()
	for nodeID := range w.vector {
		if err := w.update(nodeID, t); err != nil {
			return err
		}
	}
	return nil
}

// GetMana returns combination of Effective Base Mana 1 & 2 weighted as 50-50.
func (w *WeightedBaseManaVector) GetMana(nodeID identity.ID, optionalUpdateTime ...time.Time) (float64, time.Time, error) {
	w.Lock()
	defer w.Unlock()
	return w.getMana(nodeID, optionalUpdateTime...)
}

// GetManaMap returns mana perception of the node..
func (w *WeightedBaseManaVector) GetManaMap(optionalUpdateTime ...time.Time) (res NodeMap, t time.Time, err error) {
	w.Lock()
	defer w.Unlock()
	t = time.Now()
	if len(optionalUpdateTime) > 0 {
		t = optionalUpdateTime[0]
	}
	res = make(map[identity.ID]float64)
	for ID := range w.vector {
		var mana float64
		mana, _, err = w.getMana(ID, t)
		if err != nil {
			return nil, t, err
		}
		res[ID] = mana
	}
	return
}

// GetHighestManaNodes returns the n highest mana nodes in descending order.
// It also updates the mana values for each node.
// If n is zero, it returns all nodes.
func (w *WeightedBaseManaVector) GetHighestManaNodes(n uint) (res []Node, t time.Time, err error) {
	err = func() error {
		// don't lock the vector after this func returns
		w.Lock()
		defer w.Unlock()
		t = time.Now()
		for ID := range w.vector {
			var mana float64
			mana, _, err = w.getMana(ID, t)
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
func (w *WeightedBaseManaVector) GetHighestManaNodesFraction(p float64) (res []Node, t time.Time, err error) {
	totalMana := 0.0
	err = func() error {
		// don't lock the vector after this func returns
		w.Lock()
		defer w.Unlock()
		t = time.Now()
		for ID := range w.vector {
			var mana float64
			mana, _, err = w.getMana(ID, t)
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
func (w *WeightedBaseManaVector) SetMana(nodeID identity.ID, bm BaseMana) {
	w.Lock()
	defer w.Unlock()
	w.vector[nodeID] = bm.(*WeightedBaseMana)
}

// SetMana1 sets the mana1 (consensus) part for a node.
func (w *WeightedBaseManaVector) SetMana1(nodeID identity.ID, bm *ConsensusBaseMana) {
	w.Lock()
	defer w.Unlock()
	if _, exist := w.vector[nodeID]; !exist {
		w.vector[nodeID] = NewWeightedMana(w.weight)
	}
	w.vector[nodeID].mana1 = bm
}

// SetMana2 sets the mana2 (access) part for a node.
func (w *WeightedBaseManaVector) SetMana2(nodeID identity.ID, bm *AccessBaseMana) {
	w.Lock()
	defer w.Unlock()
	if _, exist := w.vector[nodeID]; !exist {
		w.vector[nodeID] = NewWeightedMana(w.weight)
	}
	w.vector[nodeID].mana2 = bm
}

// ForEach iterates over the vector and calls the provided callback.
func (w *WeightedBaseManaVector) ForEach(callback func(ID identity.ID, bm BaseMana) bool) {
	// lock to be on the safe side, although callback might just read
	w.Lock()
	defer w.Unlock()
	for nodeID, baseMana := range w.vector {
		if !callback(nodeID, baseMana) {
			return
		}
	}
}

// ToPersistables converts the WeightedBaseManaVector to a list of persistable mana objects.
func (w *WeightedBaseManaVector) ToPersistables() []*PersistableBaseMana {
	w.RLock()
	defer w.RUnlock()
	var result []*PersistableBaseMana
	for nodeID, bm := range w.vector {
		pbm := &PersistableBaseMana{
			ManaType:        w.Type(),
			BaseValues:      []float64{bm.mana1.BaseValue(), bm.mana2.BaseValue()},
			EffectiveValues: []float64{bm.mana1.EffectiveValue(), bm.mana2.EffectiveValue()},
			LastUpdated:     bm.LastUpdate(),
			NodeID:          nodeID,
		}
		result = append(result, pbm)
	}
	return result
}

// FromPersistable fills the WeightedBaseManaVector from persistable mana objects.
func (w *WeightedBaseManaVector) FromPersistable(p *PersistableBaseMana) (err error) {
	if p.ManaType != WeightedMana {
		err = xerrors.Errorf("persistable mana object has type %s instead of %s", p.ManaType.String(), WeightedMana.String())
		return
	}
	if len(p.BaseValues) != 2 {
		err = xerrors.Errorf("persistable mana object has %d base values instead of 2", len(p.BaseValues))
		return
	}
	if len(p.EffectiveValues) != 2 {
		err = xerrors.Errorf("persistable mana object has %d effective values instead of 2", len(p.EffectiveValues))
		return
	}
	w.Lock()
	defer w.Unlock()
	w.vector[p.NodeID] = &WeightedBaseMana{
		mana1: &ConsensusBaseMana{
			BaseMana1:          p.BaseValues[0],
			EffectiveBaseMana1: p.EffectiveValues[0],
			LastUpdated:        p.LastUpdated,
		},
		mana2: &AccessBaseMana{
			BaseMana2:          p.BaseValues[1],
			EffectiveBaseMana2: p.EffectiveValues[1],
			LastUpdated:        p.LastUpdated,
		},
		weight: w.weight,
	}
	return
}

// RemoveZeroNodes removes the zero mana nodes from the vector.
func (w *WeightedBaseManaVector) RemoveZeroNodes() {
	w.Lock()
	defer w.Unlock()
	for nodeID, baseMana := range w.vector {
		if baseMana.EffectiveValue() < MinEffectiveMana && baseMana.BaseValue() < MinBaseMana {
			delete(w.vector, nodeID)
		}
	}
}

var _ BaseManaVector = &WeightedBaseManaVector{}

//// Region Internal methods ////

// update updates the mana entries for a particular node wrt time. Not concurrency safe.
func (w *WeightedBaseManaVector) update(nodeID identity.ID, t time.Time) error {
	if _, exist := w.vector[nodeID]; !exist {
		return ErrNodeNotFoundInBaseManaVector
	}

	// a *WeightedMana contains two references, so we need to work around and copy the actual values for oldMana
	// 1. create empty *WeightedMana
	oldMana := NewWeightedMana(w.weight)
	// 2. save value of mana1 reference into temp variable
	ctmp := *w.vector[nodeID].mana1
	// 3. reference the temp variable for oldMana.mana1
	oldMana.mana1 = &ctmp
	// 2. save value of mana2 reference into temp variable
	atmp := *w.vector[nodeID].mana2
	// 3. reference the temp variable for oldMana.mana2
	oldMana.mana2 = &atmp

	if err := w.vector[nodeID].update(t); err != nil {
		return err
	}
	Events().Updated.Trigger(&UpdatedEvent{nodeID, oldMana, w.vector[nodeID], w.Type()})
	return nil
}

// getMana returns the current effective mana value. Not concurrency safe.
func (w *WeightedBaseManaVector) getMana(nodeID identity.ID, optionalUpdateTime ...time.Time) (float64, time.Time, error) {
	t := time.Now()
	if _, exist := w.vector[nodeID]; !exist {
		return 0.0, t, ErrNodeNotFoundInBaseManaVector
	}
	if len(optionalUpdateTime) > 0 {
		t = optionalUpdateTime[0]
	}
	_ = w.update(nodeID, t)

	baseMana := w.vector[nodeID]
	return baseMana.EffectiveValue(), t, nil
}
