package state

import (
	"encoding/binary"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
)

// Randomness defines the current randomness state of a DRNG instance.
type Randomness struct {
	// Round holds the current DRNG round.
	Round uint64
	// Randomness holds the current randomness as a slice of bytes.
	Randomness []byte
	// Timestamp holds the timestamp when the current randomness was received.
	Timestamp time.Time
}

// Float64 returns a float64 [0.0,1.0) representation of the randomness byte slice.
func (r Randomness) Float64() float64 {
	return float64(binary.BigEndian.Uint64(r.Randomness[:8])>>11) / (1 << 53)
}

// Committee defines the current committee state of a DRNG instance.
type Committee struct {
	// InstanceID holds the identifier of the dRAND instance.
	InstanceID uint32
	// Threshold holds the threshold of the secret sharing protocol.
	Threshold uint8
	// Identities holds the nodes' identities of the committee members.
	Identities []ed25519.PublicKey
	// DistributedPK holds the drand distributed public key.
	DistributedPK []byte
}

// State represents the state of the DRNG.
type State struct {
	randomness *Randomness
	committee  *Committee

	mutex sync.RWMutex
}

// New creates a new State with the given optional options
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

// UpdateRandomness updates the randomness of the DRNG state
func (s *State) UpdateRandomness(r *Randomness) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.randomness = r
}

// Randomness returns the randomness of the DRNG state
func (s *State) Randomness() Randomness {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if s.randomness == nil {
		return Randomness{}
	}
	return *s.randomness
}

// UpdateCommittee updates the committee of the DRNG state
func (s *State) UpdateCommittee(c *Committee) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.committee = c
}

// Committee returns the committee of the DRNG state
func (s *State) Committee() Committee {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if s.committee == nil {
		return Committee{}
	}
	return *s.committee
}
