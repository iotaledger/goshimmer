package mana // Type defines if mana is access or consensus type of mana.

type Type int

const (
	// AccessMana is mana associated with access to the network.
	AccessMana Type = iota
	// ConsensusMana is mana associated with consensus weights in the network.
	ConsensusMana
)
