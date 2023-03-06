package enginemanager_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/snapshotcreator"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledgerstate/ondiskledgerstate"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/dpos"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1"
	"github.com/iotaledger/goshimmer/packages/protocol/enginemanager"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/realitiesledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/storage/utils"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

type EngineManagerTestFramework struct {
	EngineManager *enginemanager.EngineManager

	ActiveEngine *enginemanager.EngineInstance
}

func NewEngineManagerTestFramework(t *testing.T, workers *workerpool.Group, identitiesWeights map[ed25519.PublicKey]uint64) *EngineManagerTestFramework {
	tf := &EngineManagerTestFramework{}

	ledgerProvider := realitiesledger.NewProvider(realitiesledger.WithVM(new(devnetvm.VM)))

	snapshotPath := utils.NewDirectory(t.TempDir()).Path("snapshot.bin")

	slotDuration := int64(10)
	require.NoError(t, snapshotcreator.CreateSnapshot(
		snapshotcreator.WithDatabaseVersion(protocol.DatabaseVersion),
		snapshotcreator.WithFilePath(snapshotPath),
		snapshotcreator.WithGenesisTokenAmount(1),
		snapshotcreator.WithGenesisSeed(make([]byte, 32)),
		snapshotcreator.WithPeersPublicKeys(lo.Keys(identitiesWeights)),
		snapshotcreator.WithLedgerProvider(ledgerProvider),
		snapshotcreator.WithPeersAmountsPledged(lo.Values(identitiesWeights)),
		snapshotcreator.WithAttestAll(true),
		snapshotcreator.WithGenesisUnixTime(time.Now().Unix()-slotDuration*10),
		snapshotcreator.WithSlotDuration(slotDuration),
	))

	tf.EngineManager = enginemanager.New(workers.CreateGroup("EngineManager"),
		t.TempDir(),
		protocol.DatabaseVersion,
		[]options.Option[database.Manager]{},
		[]options.Option[engine.Engine]{},
		ledgerProvider,
		ondiskledgerstate.NewProvider(),
		dpos.NewProvider(),
		mana1.NewProvider(),
	)

	var err error
	tf.ActiveEngine, err = tf.EngineManager.LoadActiveEngine()
	require.NoError(t, err)

	require.NoError(t, tf.ActiveEngine.InitializeWithSnapshot(snapshotPath))

	return tf
}
