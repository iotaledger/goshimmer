package mana

import (
	"bytes"
	"fmt"
	"sort"
	"time"

	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/identity"
)

// ManaBaseVector represents a base mana vector.
type ManaBaseVector struct {
	model.Mutable[ManaBaseVector, *ManaBaseVector, manaBaseVectorModel] `serix:"0"`
}

type manaBaseVectorModel struct {
	Type   Type                      `serix:"0"`
	Vector map[identity.ID]*ManaBase `serix:"1"`
}

// Vector returns the ManaBase vector.
func (m *ManaBaseVector) Vector() map[identity.ID]*ManaBase {
	m.RLock()
	defer m.RUnlock()
	return m.M.Vector
}

// Type returns the type of this mana vector.
func (m *ManaBaseVector) Type() Type {
	return m.M.Type
}

// Size returns the size of this mana vector.
func (m *ManaBaseVector) Size() int {
	m.RLock()
	defer m.RUnlock()
	return len(m.M.Vector)
}

// Has returns if the given node has mana defined in the vector.
func (m *ManaBaseVector) Has(nodeID identity.ID) bool {
	m.RLock()
	defer m.RUnlock()
	_, exists := m.M.Vector[nodeID]
	return exists
}

// // BuildPastBaseVector builds a consensus base mana vector from past events upto time `t`.
// // `eventLogs` is expected to be sorted chronologically.
// func (c *ConsensusBaseManaVector) BuildPastBaseVector(eventsLog []Event, t time.Time) error {
//	if m.vector == nil {
//		m.vector = make(map[identity.ID]*ConsensusBaseMana)
//	}
//	for _, _ev := range eventsLog {
//		switch _ev.Type() {
//		case EventTypePledge:
//			ev := _ev.(*PledgedEvent)
//			if ev.Time.After(t) {
//				return nil
//			}
//			if _, exist := m.vector[ev.NodeID]; !exist {
//				m.vector[ev.NodeID] = &ConsensusBaseMana{}
//			}
//			m.vector[ev.NodeID].pledge(txInfoFromPledgeEvent(ev))
//		case EventTypeRevoke:
//			ev := _ev.(*RevokedEvent)
//			if ev.Time.After(t) {
//				return nil
//			}
//			if _, exist := m.vector[ev.NodeID]; !exist {
//				m.vector[ev.NodeID] = &ConsensusBaseMana{}
//			}
//			err := m.vector[ev.NodeID].revoke(ev.Amount, ev.Time)
//			if err != nil {
//				return err
//			}
//		}
//	}
//	return nil
// }

// InitializeWithData initializes the mana vector data.
func (m *ManaBaseVector) InitializeWithData(dataByNode map[identity.ID]float64) {
	m.Lock()
	defer m.Unlock()
	for nodeID, value := range dataByNode {
		m.M.Vector[nodeID] = NewManaBase(value)
	}
}

// Book books mana for a transaction.
func (m *ManaBaseVector) Book(txInfo *TxInfo) {
	// gather events to be triggered once the lock is lifted
	var revokeEvents []*RevokedEvent
	var pledgeEvents []*PledgedEvent
	var updateEvents []*UpdatedEvent
	// only lock mana vector while we are working with it
	func() {
		m.Lock()
		defer m.Unlock()
		// first, revoke mana from previous owners
		for _, inputInfo := range txInfo.InputInfos {
			// which node did the input pledge mana to?
			oldPledgeNodeID := inputInfo.PledgeID[m.Type()]
			oldMana := m.getOldManaAndRevoke(oldPledgeNodeID, inputInfo.Amount)
			// save events for later triggering
			revokeEvents = append(revokeEvents, &RevokedEvent{
				NodeID:        oldPledgeNodeID,
				Amount:        inputInfo.Amount,
				Time:          txInfo.TimeStamp,
				ManaType:      m.Type(),
				TransactionID: txInfo.TransactionID,
				InputID:       inputInfo.InputID,
			})
			updateEvents = append(updateEvents, &UpdatedEvent{
				NodeID:   oldPledgeNodeID,
				OldMana:  &oldMana,
				NewMana:  m.M.Vector[oldPledgeNodeID],
				ManaType: m.Type(),
			})
		}
		// second, pledge mana to new nodes
		newPledgeNodeID := txInfo.PledgeID[m.Type()]
		oldMana := m.getOldManaAndPledge(newPledgeNodeID, txInfo.TotalBalance)

		pledgeEvents = append(pledgeEvents, &PledgedEvent{
			NodeID:        newPledgeNodeID,
			Amount:        txInfo.sumInputs(),
			Time:          txInfo.TimeStamp,
			ManaType:      m.Type(),
			TransactionID: txInfo.TransactionID,
		})
		updateEvents = append(updateEvents, &UpdatedEvent{
			NodeID:   newPledgeNodeID,
			OldMana:  &oldMana,
			NewMana:  m.M.Vector[newPledgeNodeID],
			ManaType: m.Type(),
		})
	}()

	m.triggerManaEvents(revokeEvents, pledgeEvents, updateEvents)
}

func (m *ManaBaseVector) triggerManaEvents(revokeEvents []*RevokedEvent, pledgeEvents []*PledgedEvent, updateEvents []*UpdatedEvent) {
	// trigger the events once we released the lock on the mana vector
	for _, ev := range revokeEvents {
		Events.Revoked.Trigger(ev)
	}
	for _, ev := range pledgeEvents {
		Events.Pledged.Trigger(ev)
	}
	for _, ev := range updateEvents {
		Events.Updated.Trigger(ev)
	}
}

// BookEpoch takes care of the booking of consensus mana for the given committed epoch.
func (m *ManaBaseVector) BookEpoch(created []*ledger.OutputWithMetadata, spent []*ledger.OutputWithMetadata) {
	var revokeEvents []*RevokedEvent
	var pledgeEvents []*PledgedEvent
	var updateEvents []*UpdatedEvent
	// only lock mana vector while we are working with it
	func() {
		m.Lock()
		defer m.Unlock()

		// first, revoke mana from previous owners
		for _, output := range spent {
			idToRevoke := m.getIDBasedOnManaType(output)
			outputIOTAs, exists := output.Output().(devnetvm.Output).Balances().Get(devnetvm.ColorIOTA)
			if !exists {
				continue
			}
			oldMana := m.getOldManaAndRevoke(idToRevoke, float64(outputIOTAs))

			// save events for later triggering
			revokeEvents = append(revokeEvents, &RevokedEvent{
				NodeID:        idToRevoke,
				Amount:        float64(outputIOTAs),
				Time:          output.CreationTime(),
				ManaType:      m.Type(),
				TransactionID: output.ID().TransactionID,
				InputID:       output.ID(),
			})
			updateEvents = append(updateEvents, &UpdatedEvent{
				NodeID:   idToRevoke,
				OldMana:  &oldMana,
				NewMana:  m.M.Vector[idToRevoke],
				ManaType: m.Type(),
			})
		}
		// second, pledge mana to new nodes
		for _, output := range created {
			idToPledge := m.getIDBasedOnManaType(output)

			outputIOTAs, exists := output.Output().(devnetvm.Output).Balances().Get(devnetvm.ColorIOTA)
			if !exists {
				continue
			}
			oldMana := m.getOldManaAndPledge(idToPledge, float64(outputIOTAs))
			pledgeEvents = append(pledgeEvents, &PledgedEvent{
				NodeID:        idToPledge,
				Amount:        float64(outputIOTAs),
				Time:          output.CreationTime(),
				ManaType:      m.Type(),
				TransactionID: output.Output().ID().TransactionID,
			})

			updateEvents = append(updateEvents, &UpdatedEvent{
				NodeID:   idToPledge,
				OldMana:  &oldMana,
				NewMana:  m.M.Vector[idToPledge],
				ManaType: m.Type(),
			})
		}
	}()
	m.triggerManaEvents(revokeEvents, pledgeEvents, updateEvents)
}

func (m *ManaBaseVector) getIDBasedOnManaType(output *ledger.OutputWithMetadata) (pledgeID identity.ID) {
	if m.Type() == ConsensusMana {
		return output.ConsensusManaPledgeID()
	}
	return output.AccessManaPledgeID()
}

func (m *ManaBaseVector) getOldManaAndRevoke(oldPledgeNodeID identity.ID, amount float64) (oldMana ManaBase) {
	if _, exist := m.M.Vector[oldPledgeNodeID]; !exist {
		// first time we see this node
		m.M.Vector[oldPledgeNodeID] = NewManaBase(0)
	}
	// save old mana
	oldMana = *m.M.Vector[oldPledgeNodeID]
	// revoke BM1
	err := m.M.Vector[oldPledgeNodeID].revoke(amount)
	if errors.Is(err, ErrBaseManaNegative) {
		panic(fmt.Sprintf("Revoking %f base mana 1 from node %s results in negative balance", amount, oldPledgeNodeID.String()))
	}
	return
}

func (m *ManaBaseVector) getOldManaAndPledge(newPledgeNodeID identity.ID, totalBalance float64) (oldMana ManaBase) {
	if _, exist := m.M.Vector[newPledgeNodeID]; !exist {
		// first time we see this node
		m.M.Vector[newPledgeNodeID] = NewManaBase(0)
	}
	// save it for proper event trigger
	oldMana = *m.M.Vector[newPledgeNodeID]
	// actually pledge and update
	m.M.Vector[newPledgeNodeID].pledge(totalBalance)
	return
}

// GetMana returns the Effective Base Mana.
func (m *ManaBaseVector) GetMana(nodeID identity.ID) (manaAmount float64, t time.Time, err error) {
	m.Lock()
	defer m.Unlock()
	manaAmount, err = m.getMana(nodeID)
	t = time.Now()
	return manaAmount, t, err
}

// GetManaMap returns mana perception of the node.
func (m *ManaBaseVector) GetManaMap() (res NodeMap, t time.Time, err error) {
	m.Lock()
	defer m.Unlock()
	t = time.Now()
	res = make(map[identity.ID]float64, len(m.M.Vector))
	for ID, val := range m.M.Vector {
		res[ID] = val.BaseValue()
	}
	return
}

// GetHighestManaNodes return the n-highest mana nodes in descending order.
// It also updates the mana values for each node.
// If n is zero, it returns all nodes.
func (m *ManaBaseVector) GetHighestManaNodes(n uint) (res []Node, t time.Time, err error) {
	t = time.Now()
	err = func() error {
		// don't lock the vector after this func returns
		m.Lock()
		defer m.Unlock()
		for ID := range m.M.Vector {
			var mana float64
			mana, err = m.getMana(ID)
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
func (m *ManaBaseVector) GetHighestManaNodesFraction(p float64) (res []Node, t time.Time, err error) {
	emptyNodeID := identity.ID{}
	totalMana := 0.0
	t = time.Now()
	err = func() error {
		// don't lock the vector after this func returns
		m.Lock()
		defer m.Unlock()
		for ID := range m.M.Vector {
			// skip the empty node ID
			if bytes.Equal(ID[:], emptyNodeID[:]) {
				continue
			}

			var mana float64
			mana, err = m.getMana(ID)
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
func (m *ManaBaseVector) SetMana(nodeID identity.ID, bm BaseMana) {
	m.Lock()
	defer m.Unlock()
	m.M.Vector[nodeID] = bm.(*ManaBase)
}

// ForEach iterates over the vector and calls the provided callback.
func (m *ManaBaseVector) ForEach(callback func(ID identity.ID, bm BaseMana) bool) {
	// lock to be on the safe side, although callback might just read
	m.Lock()
	defer m.Unlock()
	for nodeID, baseMana := range m.M.Vector {
		if !callback(nodeID, baseMana) {
			return
		}
	}
}

// ToPersistables converts the baseManaVector to a list of persistable mana objects.
func (m *ManaBaseVector) ToPersistables() []*PersistableBaseMana {
	m.RLock()
	defer m.RUnlock()
	var result []*PersistableBaseMana
	for nodeID, bm := range m.M.Vector {
		pbm := NewPersistableBaseMana(nodeID, m.Type(), []float64{bm.BaseValue()}, nil, time.Time{})
		result = append(result, pbm)
	}
	return result
}

// FromPersistable fills the BaseManaVector from persistable mana objects.
func (m *ManaBaseVector) FromPersistable(p *PersistableBaseMana) (err error) {
	if len(p.BaseValues()) != 1 {
		err = errors.Errorf("persistable mana object has %d base values instead of 1", len(p.BaseValues()))
		return
	}
	m.Lock()
	defer m.Unlock()
	m.M.Vector[p.NodeID()] = model.NewMutable[ManaBase](&manaBaseModel{Value: p.BaseValues()[0]})
	return
}

// RemoveZeroNodes removes the zero mana nodes from the vector.
func (m *ManaBaseVector) RemoveZeroNodes() {
	m.Lock()
	defer m.Unlock()
	for nodeID, baseMana := range m.M.Vector {
		if baseMana.BaseValue() == 0 {
			delete(m.M.Vector, nodeID)
		}
	}
}

var _ BaseManaVector = &ManaBaseVector{}

// // Region Internal methods ////

// getMana returns the consensus mana.
func (m *ManaBaseVector) getMana(nodeID identity.ID) (float64, error) {
	if _, exist := m.M.Vector[nodeID]; !exist {
		return 0.0, ErrNodeNotFoundInBaseManaVector
	}

	baseMana := m.M.Vector[nodeID]
	return baseMana.BaseValue(), nil
}
