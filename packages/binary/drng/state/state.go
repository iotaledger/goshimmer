package state

import (
	"encoding/binary"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
)

type Randomness struct {
	Round      uint64
	Randomness []byte
	Timestamp  time.Time
}

// Float64 returns a float64 [0.0,1.0) rapresentation of the randomness byte slice
func (r Randomness) Float64() float64 {
	return float64(binary.BigEndian.Uint64(r.Randomness[:8])>>11) / (1 << 53)
}

type Committee struct {
	InstanceID    uint32
	Threshold     uint8
	Identities    []ed25119.PublicKey
	DistributedPK []byte
}
type State struct {
	randomness *Randomness
	committee  *Committee

	mutex sync.RWMutex
}

func New(setters ...Option) *State {
	args := &Options{}

	for _, setter := range setters {
		setter(args)
	}
	return &State{
		randomness: args.Randomness,
		committee:  args.Committee,
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
	s.committee = c
}

func (s *State) Committee() Committee {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if s.committee == nil {
		return Committee{}
	}
	return *s.committee
}
