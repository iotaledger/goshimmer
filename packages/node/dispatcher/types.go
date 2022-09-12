package dispatcher

import (
	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type Fork struct {
	ForkingPoint *CachedEC

	epochCommitmentByIndex []*CachedEC
}

func NewFork(cachedEC *CachedEC) (fork *Fork) {
	return &Fork{
		ForkingPoint:           cachedEC,
		epochCommitmentByIndex: []*CachedEC{cachedEC},
	}
}

func (f *Fork) Add(cachedEC *CachedEC) {
	f.epochCommitmentByIndex[cachedEC.EI()-f.ForkingPoint.EI()] = cachedEC
}

func (f *Fork) EpochCommitmentByIndex(index epoch.Index) *CachedEC {
	return f.epochCommitmentByIndex[index-f.ForkingPoint.EI()]
}

type GlobalCache struct {
	forksByEC map[epoch.EC]ChainID
}

type CachedEC struct {
	Fork *Fork
	Prev *CachedEC
	Next *CachedEC

	*epoch.ECRecord
}

type CachedECChain struct {
	ID ChainID

	firstElement *CachedEC
	lastElement  *CachedEC
}

// ChainID uses the earliest forking point of a chain as its identifier.
type ChainID = epoch.EC
