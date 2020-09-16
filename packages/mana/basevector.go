package mana

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/identity"
)

var (
	// ErrNodeNotFoundInBaseManaVector is returned if the node is not found in the base mana vector.
	ErrNodeNotFoundInBaseManaVector = errors.New("node not present in base mana vector")
	// ErrInvalidWeightParameter is returned if an invalid weight parameter is passed.
	ErrInvalidWeightParameter = errors.New("invalid weight parameter, outside of [0,1]")
)

// BaseManaVector represents a base mana vector
type BaseManaVector struct {
	vector     map[identity.ID]*BaseMana
	vectorType Type
	sync.RWMutex
}

// NewBaseManaVector creates and returns a new base mana vector for the specified type
func NewBaseManaVector(vectorType Type) *BaseManaVector {
	return &BaseManaVector{
		vector:     make(map[identity.ID]*BaseMana),
		vectorType: vectorType,
	}
}

// ToPersistables converts the baseManaVector to a list of persistable mana objects.
func (bmv *BaseManaVector) ToPersistables() []*PersistableBaseMana {
	bmv.RLock()
	defer bmv.RUnlock()
	var result []*PersistableBaseMana
	for nodeID, bm := range bmv.vector {
		pbm := &PersistableBaseMana{
			ManaType:           bmv.vectorType,
			BaseMana1:          bm.BaseMana1,
			EffectiveBaseMana1: bm.EffectiveBaseMana1,
			BaseMana2:          bm.BaseMana2,
			EffectiveBaseMana2: bm.EffectiveBaseMana2,
			LastUpdated:        bm.LastUpdated,
			NodeID:             nodeID,
		}
		result = append(result, pbm)
	}
	return result
}

// FromPersitable fills the basemanavector from persistable mana objects.
func (bmv *BaseManaVector) FromPersitable(p *PersistableBaseMana) {
	bmv.Lock()
	defer bmv.Unlock()
	bmv.vector[p.NodeID] = &BaseMana{
		BaseMana1:          p.BaseMana1,
		EffectiveBaseMana1: p.EffectiveBaseMana1,
		BaseMana2:          p.BaseMana2,
		EffectiveBaseMana2: p.EffectiveBaseMana2,
		LastUpdated:        p.LastUpdated,
	}
}

// Type returns the type of this mana vector.
func (bmv *BaseManaVector) Type() Type {
	bmv.RLock()
	defer bmv.RUnlock()
	return bmv.vectorType
}

// Size returns the size of this mana vector.
func (bmv *BaseManaVector) Size() int {
	bmv.RLock()
	defer bmv.RUnlock()
	return len(bmv.vector)
}

// BookMana books mana for a transaction.
func (bmv *BaseManaVector) BookMana(txInfo *TxInfo) {
	bmv.Lock()
	defer bmv.Unlock()
	// first, revoke mana from previous owners
	for _, inputInfo := range txInfo.InputInfos {
		// which node did the input pledge mana to?
		pledgeNodeID := inputInfo.PledgeID[bmv.vectorType]
		// can't revoke from genesis
		emptyID := identity.ID{}
		if pledgeNodeID == emptyID {
			continue
		}
		if _, exist := bmv.vector[pledgeNodeID]; !exist {
			// first time we see this node
			bmv.vector[pledgeNodeID] = &BaseMana{}
		}
		// save old mana
		oldMana := bmv.vector[pledgeNodeID]
		// revoke BM1
		if err := bmv.vector[pledgeNodeID].revokeBaseMana1(inputInfo.Amount, txInfo.TimeStamp); err == ErrBaseManaNegative {
			panic(fmt.Sprintf("Revoking %f base mana 1 from node %s results in negative balance", inputInfo.Amount, pledgeNodeID.String()))
		}

		// trigger events
		Events().Revoked.Trigger(&RevokedEvent{pledgeNodeID, inputInfo.Amount, txInfo.TimeStamp, bmv.vectorType})
		Events().Updated.Trigger(&UpdatedEvent{pledgeNodeID, *oldMana, *bmv.vector[pledgeNodeID], bmv.vectorType})
	}
	// second, pledge mana to new nodes
	pledgeNodeID := txInfo.PledgeID[bmv.vectorType]
	if _, exist := bmv.vector[pledgeNodeID]; !exist {
		// first time we see this node
		bmv.vector[pledgeNodeID] = &BaseMana{}
	}
	// save it for proper event trigger
	oldMana := bmv.vector[pledgeNodeID]
	// actually pledge and update
	bm1Pledged, bm2Pledged := bmv.vector[pledgeNodeID].pledgeAndUpdate(txInfo)

	// trigger events
	Events().Pledged.Trigger(&PledgedEvent{pledgeNodeID, bm1Pledged, bm2Pledged, txInfo.TimeStamp, bmv.vectorType})
	Events().Updated.Trigger(&UpdatedEvent{pledgeNodeID, *oldMana, *bmv.vector[pledgeNodeID], bmv.vectorType})
}

// Update updates the mana entries for a particular node wrt time.
func (bmv *BaseManaVector) Update(nodeID identity.ID, t time.Time) error {
	bmv.Lock()
	defer bmv.Unlock()
	return bmv.update(nodeID, t)
}

// UpdateAll updates all entries in the base mana vector wrt to `t`.
func (bmv *BaseManaVector) UpdateAll(t time.Time) error {
	bmv.Lock()
	defer bmv.Unlock()
	for nodeID := range bmv.vector {
		if err := bmv.update(nodeID, t); err != nil {
			return err
		}
	}
	return nil
}

// GetWeightedMana returns the combination of Effective Base Mana 1 & 2, weighted by weight.
// mana = EBM1 * weight + EBM2 * ( 1- weight), where weight is in [0,1].
func (bmv *BaseManaVector) GetWeightedMana(nodeID identity.ID, weight float64) (float64, error) {
	bmv.Lock()
	defer bmv.Unlock()
	return bmv.getWeightedMana(nodeID, weight)
}

// GetMana returns the 50 - 50 split combination of Effective Base Mana 1 & 2.
func (bmv *BaseManaVector) GetMana(nodeID identity.ID) (float64, error) {
	bmv.Lock()
	defer bmv.Unlock()
	return bmv.getMana(nodeID)
}

// ForEach iterates over the vector and calls the provided callback.
func (bmv *BaseManaVector) ForEach(callback func(ID identity.ID, bm *BaseMana) bool) {
	// lock to be on the safe side, although callback might just read
	bmv.Lock()
	defer bmv.Unlock()
	for nodeID, baseMana := range bmv.vector {
		if !callback(nodeID, baseMana) {
			return
		}
	}
}

//GetManaMap return mana perception of the node.
func (bmv *BaseManaVector) GetManaMap() NodeMap {
	bmv.Lock()
	defer bmv.Unlock()
	res := make(map[identity.ID]float64)
	for ID := range bmv.vector {
		mana, _ := bmv.getMana(ID)
		res[ID] = mana
	}
	return res
}

// GetHighestManaNodes return the n highest mana nodes in descending order.
// It also updates the mana values for each node.
// If n is zero, it returns all nodes.
func (bmv *BaseManaVector) GetHighestManaNodes(n uint) []Node {
	var res []Node
	func() {
		// don't lock the vector after this func returns
		bmv.Lock()
		defer bmv.Unlock()
		for ID := range bmv.vector {
			mana, _ := bmv.getMana(ID)
			res = append(res, Node{
				ID:   ID,
				Mana: mana,
			})
		}
	}()

	sort.Slice(res[:], func(i, j int) bool {
		return res[i].Mana > res[j].Mana
	})

	if n == 0 || int(n) >= len(res) {
		return res[:]
	}
	return res[:n]
}

// SetMana sets the base mana for a node.
func (bmv *BaseManaVector) SetMana(nodeID identity.ID, bm *BaseMana) {
	bmv.Lock()
	defer bmv.Unlock()
	bmv.vector[nodeID] = bm
}

// Node represents a node and its mana value.
type Node struct {
	ID   identity.ID
	Mana float64
}

// NodeStr defines a node and its mana value.
// The node ID is stringified.
type NodeStr struct {
	ID   string
	Mana float64
}

// ToNodeStr converts a Node to a Nodestr
func (n Node) ToNodeStr() NodeStr {
	return NodeStr{
		ID:   n.ID.String(),
		Mana: n.Mana,
	}
}

// NodeMap is a map of nodeID and mana value.
type NodeMap map[identity.ID]float64

// NodeMapStr is a NodeMap but with string id.
type NodeMapStr map[string]float64

// ToNodeStrList converts a NodeMap to list of NodeStr.
func (n NodeMap) ToNodeStrList() []NodeStr {
	var list []NodeStr
	for ID, val := range n {
		list = append(list, NodeStr{
			ID:   ID.String(),
			Mana: val,
		})
	}
	return list
}

// GetPercentile returns the top percentile the node belongs to relative to the network in terms of mana.
func (n NodeMap) GetPercentile(node identity.ID) (float64, error) {
	if len(n) == 0 {
		return 0, nil
	}
	value, ok := n[node]
	if !ok {
		return 0, ErrNodeNotFoundInBaseManaVector
	}
	nBelow := 0.0
	for _, val := range n {
		if val < value {
			nBelow++
		}
	}

	return (nBelow / float64(len(n))) * 100, nil
}

//// Region Internal methods ////

// update updates the mana entries for a particular node wrt time. Not concurrency safe.
func (bmv *BaseManaVector) update(nodeID identity.ID, t time.Time) error {
	if _, exist := bmv.vector[nodeID]; !exist {
		return ErrNodeNotFoundInBaseManaVector
	}
	oldMana := *bmv.vector[nodeID]
	if err := bmv.vector[nodeID].update(t); err != nil {
		return err
	}
	Events().Updated.Trigger(&UpdatedEvent{nodeID, oldMana, *bmv.vector[nodeID], bmv.vectorType})
	return nil
}

// getWeightedMana returns the combination of Effective Base Mana 1 & 2, weighted by weight.
// mana = EBM1 * weight + EBM2 * ( 1- weight), where weight is in [0,1]. Not concurrency safe.
func (bmv *BaseManaVector) getWeightedMana(nodeID identity.ID, weight float64) (float64, error) {
	if _, exist := bmv.vector[nodeID]; !exist {
		return 0.0, fmt.Errorf("node %s not found: %w", nodeID.String(), ErrNodeNotFoundInBaseManaVector)
	}
	if weight < 0.0 || weight > 1.0 {
		return 0.0, ErrInvalidWeightParameter
	}
	_ = bmv.update(nodeID, time.Now())
	baseMana := bmv.vector[nodeID]
	return baseMana.EffectiveBaseMana1*weight + baseMana.EffectiveBaseMana2*(1-weight), nil
}

// getMana returns the 50 - 50 split combination of Effective Base Mana 1 & 2. Not concurrency safe.
func (bmv *BaseManaVector) getMana(nodeID identity.ID) (float64, error) {
	return bmv.getWeightedMana(nodeID, 0.5)
}
