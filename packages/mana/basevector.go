package mana

import (
	"time"

	"golang.org/x/xerrors"

	"github.com/iotaledger/hive.go/identity"
)

type BaseManaVector interface {
	// Type returns the type of the base mana vector (access/consensus).
	Type() Type
	// Size returns the size of the base mana vector.
	Size() int
	// Has tells if a certain node is present in the base mana vactor.
	Has(identity.ID) bool
	// Book books mana into the base mana vector.
	Book(*TxInfo)
	// Update updates the mana entries for a particular node wrt time.
	Update(identity.ID, time.Time) error
	// UpdateAll updates all entries in the base mana vector wrt to time.
	UpdateAll(time.Time) error
	// GetMana returns the mana value of a node with default weights.
	GetMana(identity.ID) (float64, error)
	// GetManaMap returns the map derived from the vector.
	GetManaMap() (NodeMap, error)
	// GetHighestManaNodes returns the n highest mana nodes in descending order.
	GetHighestManaNodes(uint) ([]Node, error)
	// SetMana sets the base mana for a node.
	SetMana(identity.ID, BaseMana)
	// ForEach executes a callback function for each entry in the vector.
	ForEach(func(identity.ID, BaseMana) bool)
	// ToPersistables converts the BaseManaVector to a list of persistable mana objects.
	ToPersistables() []*PersistableBaseMana
	// FromPersistable fills the BaseManaVector from persistable mana objects.
	FromPersistable(*PersistableBaseMana)
}

// NewBaseManaVector creates and returns a new base mana vector for the specified type.
func NewBaseManaVector(vectorType Type) (BaseManaVector, error) {
	switch vectorType {
	case AccessMana:
		return &AccessBaseManaVector{
			vector: make(map[identity.ID]*AccessBaseMana),
		}, nil
	case ConsensusMana:
		return &ConsensusBaseManaVector{
			vector: make(map[identity.ID]*ConsensusBaseMana),
		}, nil
	default:
		return nil, xerrors.Errorf("error while creating base mana vector with type %d: %w", vectorType, ErrUnknownType)
	}
}
