package state

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
)

type Randomness struct {
	Round      uint64
	Randomness []byte
	Timestamp  time.Time
}

type Committee struct {
	InstanceID    uint32
	Threshold     uint8
	Identities    []ed25119.PublicKey
	DistributedPK []byte
}
type State struct {
	randomness *Randomness
	committe   *Committee

	mutex sync.RWMutex
}

func New(setters ...Option) *State {
	args := &Options{}

	for _, setter := range setters {
		setter(args)
	}
	return &State{
		randomness: args.Randomness,
		committe:   args.Committee,
	}
}

func (s *State) SetRandomness(r *Randomness) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.randomness = r
}

func (s *State) Randomness() Randomness {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if s.randomness == nil {
		return Randomness{}
	}
	return *s.randomness
}

func (s *State) SetCommittee(c *Committee) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.committe = c
}

func (s *State) Committee() Committee {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if s.committe == nil {
		return Committee{}
	}
	return *s.committe
}
