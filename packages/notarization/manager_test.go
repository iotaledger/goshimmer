package notarization

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/tangle"
)

func TestNewManager(t *testing.T) {
	m := NewManager(NewEpochManager(), NewEpochCommitmentFactory(), tangle.NewTestTangle())
	assert.NotNil(t, m)
}

func TestManager_PendingBranchesCount(t *testing.T) {
	m := NewManager(NewEpochManager(), NewEpochCommitmentFactory(), tangle.NewTestTangle())
	m.pendingBranchesCount[3] = 3
	assert.Equal(t, uint64(3), m.PendingBranchesCount(3))
}

func TestManager_IsCommittable(t *testing.T) {
	m := testNotarizationManager()
	eci := ECI(5)
	m.pendingBranchesCount[eci] = 0
	// not old enough
	assert.False(t, m.IsCommittable(eci))

	eci = ECI(1)
	m.pendingBranchesCount[eci] = 1
	// old enough but pbc > 0
	assert.False(t, m.IsCommittable(eci))
	m.pendingBranchesCount[eci] = 0
	// old enough and pbc > 0
	assert.True(t, m.IsCommittable(eci))
}

func TestManager_GetLatestEC(t *testing.T) {
	m := testNotarizationManager()
	// epoch ages (in mins) since genesis [25,20,15,10,5]
	for i := 0; i <= 5; i++ {
		m.pendingBranchesCount[ECI(i)] = uint64(i)
		m.epochCommitmentFactory.InsertTangleLeaf(ECI(i), tangle.EmptyMessageID)
	}

	commitment := m.GetLatestEC()
	// only epoch 0 has pbc = 0
	assert.Equal(t, ECI(0), commitment.ECI)

	m.pendingBranchesCount[4] = 0
	commitment = m.GetLatestEC()
	// epoch 4 has pbc = 0 but is not old enough
	assert.Equal(t, ECI(0), commitment.ECI)

	m.pendingBranchesCount[2] = 0
	commitment = m.GetLatestEC()
	// epoch 2 has pbc=0 and is old enough
	assert.Equal(t, ECI(2), commitment.ECI)
}

func testNotarizationManager() *Manager {
	t := time.Now().Add(-25 * time.Minute).Unix()
	interval := int64(5 * 60)
	return NewManager(NewEpochManager(GenesisTime(t), Interval(interval)), NewEpochCommitmentFactory(), tangle.NewTestTangle(), MinCommitableEpochAge(10*time.Minute))
}
