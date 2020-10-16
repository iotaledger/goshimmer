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
		return "Access"
	case ConsensusMana:
		return "Consensus"
	default:
		return "Unknown"
	}
}

// TypeFromString parses a string and returns the type of mana it defines.
func TypeFromString(stringType string) (Type, error) {
	switch stringType {
	case "Access":
		return AccessMana, nil
	case "Consensus":
		return ConsensusMana, nil
	default:
		return 999, ErrUnknownManaType
	}
}
