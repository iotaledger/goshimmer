package mana // Type defines if mana is access or consensus type of mana.

// Type is the mana type.
type Type byte

const (
	// AccessMana is mana associated with access to the network.
	AccessMana Type = iota
	// ConsensusMana is mana associated with consensus weights in the network.
	ConsensusMana
	// WeightedMana is a weighted combination of Mana 1 (consensus) and Mana 2 (access) for research purposes.
	WeightedMana
	// ResearchAccess is a special type of WeightedMana, that targets access pledges.
	ResearchAccess
	// ResearchConsensus is a special type of WeightedMana, that targets consensus pledges.
	ResearchConsensus
)

// String returns a string representation of the type of mana.
func (t Type) String() string {
	switch t {
	case AccessMana:
		return "Access"
	case ConsensusMana:
		return "Consensus"
	case WeightedMana:
		return "Weighted"
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
	case "Weighted":
		return WeightedMana, nil
	default:
		return 255, ErrUnknownManaType
	}
}
