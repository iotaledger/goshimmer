package mana // Type defines if mana is access or consensus type of mana.

// Type is the mana type.
type Type int

const (
	// AccessMana is mana associated with access to the network.
	AccessMana Type = iota
	// ConsensusMana is mana associated with consensus weights in the network.
	ConsensusMana
)

// String returns a string representation of the type of mana.
func (t Type) String() string {
	switch t {
	case AccessMana:
		return "Access Mana"
	case ConsensusMana:
		return "Consensus Mana"
	default:
		return "Unknown Mana Type"
	}
}

const (
	// OnlyMana1 takes only EBM1 into account when getting the mana values.
	OnlyMana1 float64 = 0
	// OnlyMana2 takes only EBM2 into account when getting the mana values.
	OnlyMana2 float64 = 1.0
	// Mixed takes both EBM1 and EBM2 into account (50-50) when getting the mana values.
	Mixed float64 = 0.5
)
