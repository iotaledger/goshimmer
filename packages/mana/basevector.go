package mana

import (
	"time"

	"github.com/iotaledger/hive.go/generics/model"
	"github.com/iotaledger/hive.go/identity"
)

// BaseManaVector is an interface for vectors that store base mana values of nodes in the network.
type BaseManaVector interface {
	// Type returns the type of the base mana vector (access/consensus).
	Type() Type
	// Size returns the size of the base mana vector.
	Size() int
	// Has tells if a certain node is present in the base mana vactor.
	Has(identity.ID) bool
	// LoadSnapshot loads the initial mana state into the base mana vector.
	LoadSnapshot(map[identity.ID]*SnapshotNode)
	// Book books mana into the base mana vector.
	Book(*TxInfo)
	// GetMana returns the mana value of a node with default weights.
	GetMana(identity.ID) (float64, time.Time, error)
	// GetManaMap returns the map derived from the vector.
	GetManaMap() (NodeMap, time.Time, error)
	// GetHighestManaNodes returns the n highest mana nodes in descending order.
	GetHighestManaNodes(uint) ([]Node, time.Time, error)
	// GetHighestManaNodesFraction returns the highest mana that own 'p' percent of total mana.
	GetHighestManaNodesFraction(p float64) ([]Node, time.Time, error)
	// SetMana sets the base mana for a node.
	SetMana(identity.ID, BaseMana)
	// ForEach executes a callback function for each entry in the vector.
	ForEach(func(identity.ID, BaseMana) bool)
	// ToPersistables converts the BaseManaVector to a list of persistable mana objects.
	ToPersistables() []*PersistableBaseMana
	// FromPersistable fills the BaseManaVector from persistable mana objects.
	FromPersistable(*PersistableBaseMana) error
	// RemoveZeroNodes removes all zero mana nodes from the mana vector.
	RemoveZeroNodes()
}

// NewBaseManaVector creates and returns a new base mana vector for the specified type.
func NewBaseManaVector() BaseManaVector {
	return model.NewMutable[ManaBaseVector](&manaBaseVectorModel{Vector: make(map[identity.ID]*ManaBase)})
}
