package sybilprotection

type SybilProtection interface {
	Weights() (weights *Weights)
	Validators() (validators *WeightedSet)
	InitModule()
}
