package notarization

import (
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

func TestNewManager(t *testing.T) {
	testTangle := tangle.NewTestTangle()
	vm := new(devnetvm.VM)
	m := NewManager(NewEpochManager(), NewEpochCommitmentFactory(testTangle.Options.Store, vm), testTangle)
	assert.NotNil(t, m)
}

func TestManager_PendingConflictsCount(t *testing.T) {
	testTangle := tangle.NewTestTangle()
	vm := new(devnetvm.VM)
	m := NewManager(NewEpochManager(), NewEpochCommitmentFactory(testTangle.Options.Store, vm), testTangle)
	m.pendingConflictsCount[3] = 3
	assert.Equal(t, uint64(3), m.PendingConflictsCount(3))
}

func TestManager_IsCommittable(t *testing.T) {
	m := testNotarizationManager()
	ei := epoch.EI(5)
	m.pendingConflictsCount[ei] = 0
	// not old enough
	assert.False(t, m.IsCommittable(ei))

	ei = epoch.EI(1)
	m.pendingConflictsCount[ei] = 1
	// old enough but pbc > 0
	assert.False(t, m.IsCommittable(ei))
	m.pendingConflictsCount[ei] = 0
	// old enough and pbc > 0
	assert.True(t, m.IsCommittable(ei))
}

func TestManager_GetLatestEC(t *testing.T) {
	m := testNotarizationManager()
	// epoch ages (in mins) since genesis [25,20,15,10,5]
	for i := 0; i <= 5; i++ {
		m.pendingConflictsCount[epoch.EI(i)] = uint64(i)
		err := m.epochCommitmentFactory.InsertTangleLeaf(epoch.EI(i), tangle.EmptyMessageID)
		require.NoError(t, err)
	}

	commitment := m.GetLatestEC()
	// only epoch 0 has pbc = 0
	assert.Equal(t, epoch.EI(0), commitment.EI)

	m.pendingConflictsCount[4] = 0
	commitment = m.GetLatestEC()
	// epoch 4 has pbc = 0 but is not old enough
	assert.Equal(t, epoch.EI(0), commitment.EI)

	m.pendingConflictsCount[2] = 0
	commitment = m.GetLatestEC()
	// epoch 2 has pbc=0 and is old enough
	assert.Equal(t, epoch.EI(2), commitment.EI)
}

func testNotarizationManager() *Manager {
	t := time.Now().Add(-25 * time.Minute).Unix()
	testTangle := tangle.NewTestTangle()
	interval := int64(5 * 60)
	vm := new(devnetvm.VM)
	return NewManager(NewEpochManager(GenesisTime(t), Interval(interval)), NewEpochCommitmentFactory(testTangle.Options.Store, vm), testTangle, MinCommittableEpochAge(10*time.Minute))
}
