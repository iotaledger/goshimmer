package notarization

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/conflictdag"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

func TestNewManager(t *testing.T) {
	testTangle := tangle.NewTestTangle()
	vm := new(devnetvm.VM)
	m := NewManager(NewEpochManager(), NewEpochCommitmentFactory(testTangle.Options.Store, vm, testTangle), testTangle)
	assert.NotNil(t, m)
}

func TestManager_PendingConflictsCount(t *testing.T) {
	testTangle := tangle.NewTestTangle()
	vm := new(devnetvm.VM)
	m := NewManager(NewEpochManager(), NewEpochCommitmentFactory(testTangle.Options.Store, vm, testTangle), testTangle)
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

	commitment, err := m.GetLatestEC()
	assert.NoError(t, err)
	// only epoch 0 has pbc = 0
	assert.Equal(t, epoch.EI(0), commitment.EI)

	m.pendingConflictsCount[4] = 0
	commitment, err = m.GetLatestEC()
	assert.NoError(t, err)
	// epoch 4 has pbc = 0 but is not old enough
	assert.Equal(t, epoch.EI(0), commitment.EI)

	m.pendingConflictsCount[2] = 0
	commitment, err = m.GetLatestEC()
	assert.NoError(t, err)
	// epoch 2 has pbc=0 and is old enough
	assert.Equal(t, epoch.EI(2), commitment.EI)
}

func TestManager_UpdateTangleTree(t *testing.T) {
	var epochInterval int64 = 1
	var minCommittable time.Duration = 0
	genesisTime := time.Now()

	testTangle := tangle.NewTestTangle()
	vm := new(devnetvm.VM)
	nodes := make(map[string]*identity.Identity)
	for _, node := range []string{"A", "B"} {
		nodes[node] = identity.GenerateIdentity()
	}

	ecFactory := NewEpochCommitmentFactory(testTangle.Options.Store, vm, testTangle)
	m := NewManager(NewEpochManager(Interval(epochInterval), GenesisTime(genesisTime.Unix())), ecFactory, testTangle, MinCommittableEpochAge(minCommittable))
	registerToTangleEvents(m, testTangle)
	testFramework := tangle.NewMessageTestFramework(testTangle, tangle.WithGenesisOutput("A", 500), tangle.WithGenesisOutput("B", 500))
	testEventMock := NewEventMock(t, m, ecFactory)

	// ISSUE Message1
	{
		ecRecord, err := m.GetLatestEC()
		require.NoError(t, err)

		testFramework.CreateMessage("Message1", tangle.WithStrongParents("Genesis"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithECRecord(ecRecord))

		// TODO: check if leaf is inserted
	}

	//  ISSUE Message2, issuing time epoch 1
	{
		ecRecord, err := m.GetLatestEC()
		require.NoError(t, err)
		issuingTime := genesisTime.Add(time.Duration(1 * epochInterval))
		testFramework.CreateMessage("Message2", tangle.WithStrongParents("Genesis"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithIssuingTime(issuingTime), tangle.WithECRecord(ecRecord))

		// TODO: check if leaf is inserted

		testEventMock.Expect("NewCommitmentTreesCreated", epoch.EI(1))

	}

	//  ISSUE Message3, issuing time epoch 2
	{
		ecRecord, err := m.GetLatestEC()
		require.NoError(t, err)
		issuingTime := genesisTime.Add(time.Duration(2 * epochInterval))
		testFramework.CreateMessage("Message2", tangle.WithStrongParents("Genesis"), tangle.WithIssuer(nodes["A"].PublicKey()), tangle.WithIssuingTime(issuingTime), tangle.WithECRecord(ecRecord))

		// TODO: check if leaf is inserted

		testEventMock.Expect("NewCommitmentTreesCreated", epoch.EI(2))
		testEventMock.Expect("EpochCommitted", epoch.EI(2))
	}
}

func testNotarizationManager() *Manager {
	t := time.Now().Add(-25 * time.Minute).Unix()
	testTangle := tangle.NewTestTangle()
	interval := int64(5 * 60)
	vm := new(devnetvm.VM)
	return NewManager(NewEpochManager(GenesisTime(t), Interval(interval)), NewEpochCommitmentFactory(testTangle.Options.Store, vm, testTangle), testTangle, MinCommittableEpochAge(10*time.Minute))
}

func registerToTangleEvents(m *Manager, testTangle *tangle.Tangle) {
	testTangle.ConfirmationOracle.Events().MessageConfirmed.Attach(event.NewClosure(func(event *tangle.MessageConfirmedEvent) {
		testTangle.Storage.Message(event.Message.ID()).Consume(func(msg *tangle.Message) {
			m.OnMessageConfirmed(msg)
		})
	}))
	testTangle.ConfirmationOracle.Events().MessageOrphaned.Attach(event.NewClosure(func(event *tangle.MessageConfirmedEvent) {
		m.OnMessageOrphaned(event.Message)
	}))
	testTangle.Ledger.Events.TransactionConfirmed.Attach(event.NewClosure(func(event *ledger.TransactionConfirmedEvent) {
		testTangle.Ledger.Storage.CachedTransaction(event.TransactionID).Consume(func(t utxo.Transaction) {
			m.OnTransactionConfirmed(t.(*devnetvm.Transaction))
		})
	}))
	testTangle.Ledger.Events.TransactionInclusionUpdated.Attach(event.NewClosure(func(event *ledger.TransactionInclusionUpdatedEvent) {
		m.OnTransactionInclusionUpdated(event)
	}))

	testTangle.Ledger.ConflictDAG.Events.BranchConfirmed.Attach(event.NewClosure(func(event *conflictdag.BranchConfirmedEvent[utxo.TransactionID]) {
		m.OnBranchConfirmed(event.ID)
	}))
	testTangle.Ledger.ConflictDAG.Events.ConflictCreated.Attach(event.NewClosure(func(event *conflictdag.ConflictCreatedEvent[utxo.TransactionID, utxo.OutputID]) {
		m.OnBranchCreated(event.ID)
	}))
	testTangle.Ledger.ConflictDAG.Events.BranchRejected.Attach(event.NewClosure(func(event *conflictdag.BranchRejectedEvent[utxo.TransactionID]) {
		m.OnBranchRejected(event.ID)
	}))
}
