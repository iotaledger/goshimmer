package mana

import (
	"errors"
	"fmt"
	"sort"
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
	bmv.vector[p.NodeID] = &BaseMana{
		BaseMana1:          p.BaseMana1,
		EffectiveBaseMana1: p.EffectiveBaseMana1,
		BaseMana2:          p.BaseMana2,
		EffectiveBaseMana2: p.EffectiveBaseMana2,
		LastUpdated:        p.LastUpdated,
	}
}

// BookMana books mana for a transaction.
func (bmv *BaseManaVector) BookMana(txInfo *TxInfo) {
	// first, revoke mana from previous owners
	for _, inputInfo := range txInfo.InputInfos {
		// which node did the input pledge mana to?
		pledgeNodeID := inputInfo.PledgeID[bmv.vectorType]
		if _, exist := bmv.vector[pledgeNodeID]; !exist {
			// first time we see this node
			bmv.vector[pledgeNodeID] = &BaseMana{}
		}
		// save old mana
		oldMana := bmv.vector[pledgeNodeID]
		// revoke BM1
		bmv.vector[pledgeNodeID].revokeBaseMana1(inputInfo.Amount, txInfo.TimeStamp)

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
	if _, exist := bmv.vector[nodeID]; !exist {
		return ErrNodeNotFoundInBaseManaVector
	}
	oldMana := bmv.vector[nodeID]
	if err := bmv.vector[nodeID].update(t); err != nil {
		return err
	}
	Events().Updated.Trigger(&UpdatedEvent{nodeID, *oldMana, *bmv.vector[nodeID], bmv.vectorType})
	return nil
}

// UpdateAll updates all entries in the base mana vector wrt to `t`.
func (bmv *BaseManaVector) UpdateAll(t time.Time) error {
	for nodeID := range bmv.vector {
		if err := bmv.Update(nodeID, t); err != nil {
			return err
		}
	}
	return nil
}

// GetWeightedMana returns the combination of Effective Base Mana 1 & 2, weighted by weight.
// mana = EBM1 * weight + EBM2 * ( 1- weight), where weight is in [0,1].
func (bmv *BaseManaVector) GetWeightedMana(nodeID identity.ID, weight float64) (float64, error) {
	if _, exist := bmv.vector[nodeID]; !exist {
		return 0.0, fmt.Errorf("node %s not found: %w", nodeID.String(), ErrNodeNotFoundInBaseManaVector)
	}
	if weight < 0.0 || weight > 1.0 {
		return 0.0, ErrInvalidWeightParameter
	}
	_ = bmv.Update(nodeID, time.Now())
	baseMana := bmv.vector[nodeID]
	return baseMana.EffectiveBaseMana1*weight + baseMana.EffectiveBaseMana2*(1-weight), nil
}

// GetMana returns the 50 - 50 split combination of Effective Base Mana 1 & 2.
func (bmv *BaseManaVector) GetMana(nodeID identity.ID) (float64, error) {
	return bmv.GetWeightedMana(nodeID, 0.5)
}

// ForEach iterates over the vector and calls the provided callback.
func (bmv *BaseManaVector) ForEach(callback func(ID identity.ID, bm *BaseMana) bool) {
	for nodeID, baseMana := range bmv.vector {
		if !callback(nodeID, baseMana) {
			return
		}
	}
}

//GetManaMap return mana perception of the node.
func (bmv *BaseManaVector) GetManaMap() NodeMap {
	res := make(map[identity.ID]float64)
	for ID := range bmv.vector {
		mana, _ := bmv.GetMana(ID)
		res[ID] = mana
	}
	return res
}

// GetHighestManaNodes return the n highest mana nodes in descending order.
// It also updates the mana values for each node.
func (bmv *BaseManaVector) GetHighestManaNodes(n uint) []Node {
	var res []Node
	for ID := range bmv.vector {
		mana, _ := bmv.GetMana(ID)
		res = append(res, Node{
			ID:   ID,
			Mana: mana,
		})
	}

	sort.Slice(res[:], func(i, j int) bool {
		return res[i].Mana > res[j].Mana
	})

	if int(n) <= len(res) {
		return res[:n]
	}
	return res[:]
}

// SetMana sets the base mana for a node
func (bmv *BaseManaVector) SetMana(nodeID identity.ID, bm *BaseMana) {
	bmv.vector[nodeID] = bm
}

// Node represents a node and its mana value.
type Node struct {
	ID   identity.ID
	Mana float64
}

// NodeMap is a map of nodeID and mana value.
type NodeMap map[identity.ID]float64
