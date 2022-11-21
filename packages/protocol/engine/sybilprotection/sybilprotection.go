package sybilprotection

type SybilProtection interface {
	Validators() (validators *WeightedSet)
	Weights() (weightedActors *Weights)
	InitModule()
}
