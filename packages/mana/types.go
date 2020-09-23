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
