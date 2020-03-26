package state

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
	"github.com/stretchr/testify/require"
)

func dummyRandomness() *Randomness {
	return &Randomness{
		Round:      0,
		Randomness: []byte{},
	}
}

func dummyCommittee() *Committee {
	return &Committee{
		InstanceID:    0,
		Threshold:     0,
		Identities:    []ed25119.PublicKey{},
		DistributedPK: []byte{},
	}
}

func TestState(t *testing.T) {
	// constructor
	stateTest := New(SetCommittee(dummyCommittee()), SetRandomness(dummyRandomness()))
	require.Equal(t, *dummyRandomness(), stateTest.Randomness())
	require.Equal(t, *dummyCommittee(), stateTest.Committee())

	// committee setters - getters
	newCommittee := &Committee{1, 1, []ed25119.PublicKey{}, []byte{11}}
	stateTest.SetCommittee(newCommittee)
	require.Equal(t, *newCommittee, stateTest.Committee())

	// randomness setters - getters
	newRandomness := &Randomness{1, []byte{123}, time.Now()}
	stateTest.SetRandomness(newRandomness)
	require.Equal(t, *newRandomness, stateTest.Randomness())
}

func TestFloat64(t *testing.T) {

	max := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	r := &Randomness{1, max, time.Now()}
	stateTest := New(SetRandomness(r))
	require.Equal(t, 0.9999999999999999, stateTest.Randomness().Float64())

}
