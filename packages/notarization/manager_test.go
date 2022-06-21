package notarization

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/tangle"
)

func TestNewManager(t *testing.T) {
	testTangle := tangle.NewTestTangle()
	vm := new(devnetvm.VM)
	m := NewManager(epoch.NewManager(), NewEpochCommitmentFactory(testTangle.Options.Store, vm, testTangle), testTangle)
	assert.NotNil(t, m)
}

func TestManager_PendingConflictsCount(t *testing.T) {
	testTangle := tangle.NewTestTangle()
	vm := new(devnetvm.VM)
	m := NewManager(epoch.NewManager(), NewEpochCommitmentFactory(testTangle.Options.Store, vm, testTangle), testTangle)
	m.pendingConflictsCount[3] = 3
	assert.Equal(t, uint64(3), m.PendingConflictsCount(3))
}

func TestManager_IsCommittable(t *testing.T) {
	m := testNotarizationManager()
	ei := epoch.Index(5)
	m.pendingConflictsCount[ei] = 0
	// not old enough
	assert.False(t, m.IsCommittable(ei))

	ei = epoch.Index(1)
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
		m.pendingConflictsCount[epoch.Index(i)] = uint64(i)
		err := m.epochCommitmentFactory.InsertTangleLeaf(epoch.Index(i), tangle.EmptyMessageID)
		require.NoError(t, err)
	}

	commitment, err := m.GetLatestEC()
	assert.NoError(t, err)
	// only epoch 0 has pbc = 0
	assert.Equal(t, epoch.Index(0), commitment.EI)

	m.pendingConflictsCount[4] = 0
	commitment, err = m.GetLatestEC()
	assert.NoError(t, err)
	// epoch 4 has pbc = 0 but is not old enough
	assert.Equal(t, epoch.Index(0), commitment.EI)

	m.pendingConflictsCount[2] = 0
	commitment, err = m.GetLatestEC()
	assert.NoError(t, err)
	// epoch 2 has pbc=0 and is old enough
	assert.Equal(t, epoch.Index(2), commitment.EI)
}

func testNotarizationManager() *Manager {
	t := time.Now().Add(-25 * time.Minute).Unix()
	testTangle := tangle.NewTestTangle()
	interval := 5 * time.Minute
	vm := new(devnetvm.VM)
	return NewManager(epoch.NewManager(epoch.GenesisTime(t), epoch.Duration(interval)), NewEpochCommitmentFactory(testTangle.Options.Store, vm, testTangle), testTangle, MinCommittableEpochAge(10*time.Minute))
}
