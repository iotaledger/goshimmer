package creator

/*

import (
	"time"

	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types"
	"github.com/iotaledger/hive.go/core/types/confirmation"

	"github.com/iotaledger/goshimmer/packages/core/activitylog"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/models"

	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/core/chainstorage"
	"github.com/iotaledger/goshimmer/packages/core/snapshot"
)

// CreateSnapshot creates a new snapshot. Genesis is defined by genesisTokenAmount and seedBytes, it is pledged to the
// empty nodeID. The amount to pledge to each node is defined by nodesToPledge map, the funds of each pledge is burned.
// pledge funds
// | Pledge | Funds        |
// | ------ | ------------ |
// | empty  | genesisSeed  |
// | node1  | empty/burned |
// | node2  | empty/burned |
func CreateSnapshot(snapshotFileName string, genesisTokenAmount uint64, genesisSeedBytes []byte, nodesToPledge map[identity.ID]uint64) (err error) {
	now := time.Now()

	headerProd := func() (header *ledger.SnapshotHeader, err error) {
		header = &ledger.SnapshotHeader{
			FullEpochIndex: 0,
			DiffEpochIndex: 0,
			LatestECRecord: commitment.New(0, commitment.ID{}, types.Identifier{}, genesisTokenAmount),
		}

		return
	}

	sepsProd := func() (sep *snapshot.SolidEntryPoints) {
		return &snapshot.SolidEntryPoints{EI: epoch.Index(0), Seps: make([]models.BlockID, 0)}
	}

	epochDiffsProd := func() (diffs *ledger.EpochDiff) {
		outputs := make([]*chainstorage.OutputWithMetadata, 0)
		diffs = ledger.NewEpochDiff(outputs, outputs)
		return
	}

	// prepare outputsWithMetadata
	outputsWithMetadata := make([]*chainstorage.OutputWithMetadata, 0)
	output, outputMetadata := createOutput(seed.NewSeed(genesisSeedBytes).Address(0).Address(), genesisTokenAmount, identity.ID{}, now)
	outputsWithMetadata = append(outputsWithMetadata, chainstorage.NewOutputWithMetadata(0, output.ID(), output, outputMetadata.CreationTime(), outputMetadata.ConsensusManaPledgeID(), outputMetadata.AccessManaPledgeID()))

	// prepare activity log
	epochActivity := activitylog.NewSnapshotEpochActivity()
	epochActivity[epoch.Index(0)] = activitylog.NewSnapshotNodeActivity()

	for nodeID, value := range nodesToPledge {
		// pledge to ID but send funds to random address
		output, outputMetadata = createOutput(devnetvm.NewED25519Address(ed25519.GenerateKeyPair().PublicKey), value, nodeID, now)
		outputsWithMetadata = append(outputsWithMetadata, chainstorage.NewOutputWithMetadata(0, output.ID(), output, outputMetadata.CreationTime(), outputMetadata.ConsensusManaPledgeID(), outputMetadata.AccessManaPledgeID()))
		epochActivity[epoch.Index(0)].SetNodeActivity(nodeID, 1)
	}

	i := 0

	utxoStatesProd := func() *chainstorage.OutputWithMetadata {
		if i == len(outputsWithMetadata) {
			return nil
		}

		o := outputsWithMetadata[i]
		i++

		return o
	}

	activityLogProd := func() activitylog.SnapshotEpochActivity {
		return epochActivity
	}

	_, err = snapshot.CreateSnapshot(snapshotFileName, headerProd, sepsProd, utxoStatesProd, epochDiffsProd, activityLogProd)

	return
}

// CreateSnapshotForIntegrationTest creates a new snapshot. Genesis is defined by genesisTokenAmount and seedBytes, it
// is pledged to the node that is derived from the same seed. The amount to pledge to each node is defined by
// nodesToPledge map (seedBytes->amount), the funds of each pledge is sent to the same seed.
// | Pledge      | Funds       |
// | ----------- | ----------- |
// | genesisSeed | genesisSeed |
// | node1       | node1       |
// | node2       | node2       |
func CreateSnapshotForIntegrationTest(snapshotFileName string, genesisTokenAmount uint64, genesisSeedBytes []byte, genesisNodePledge []byte, nodesToPledge map[[32]byte]uint64) (err error) {
	now := time.Now()
	outputsWithMetadata := make([]*chainstorage.OutputWithMetadata, 0)

	// prepare activity log
	epochActivity := activitylog.NewSnapshotEpochActivity()
	epochActivity[epoch.Index(0)] = activitylog.NewSnapshotNodeActivity()

	// This is the same seed used to derive the faucet ID.
	genesisPledgeID := identity.New(ed25519.PrivateKeyFromSeed(genesisNodePledge).Public()).ID()
	output, outputMetadata := createOutput(seed.NewSeed(genesisSeedBytes).Address(0).Address(), genesisTokenAmount, genesisPledgeID, now)
	outputsWithMetadata = append(outputsWithMetadata, chainstorage.NewOutputWithMetadata(0, output.ID(), output, outputMetadata.CreationTime(), outputMetadata.ConsensusManaPledgeID(), outputMetadata.AccessManaPledgeID()))
	epochActivity[epoch.Index(0)].SetNodeActivity(genesisPledgeID, 1)

	for nodeSeedBytes, value := range nodesToPledge {
		nodeID := identity.New(ed25519.PrivateKeyFromSeed(nodeSeedBytes[:]).Public()).ID()
		output, outputMetadata = createOutput(seed.NewSeed(nodeSeedBytes[:]).Address(0).Address(), value, nodeID, now)
		outputsWithMetadata = append(outputsWithMetadata, chainstorage.NewOutputWithMetadata(0, output.ID(), output, outputMetadata.CreationTime(), outputMetadata.ConsensusManaPledgeID(), outputMetadata.AccessManaPledgeID()))
		epochActivity[epoch.Index(0)].SetNodeActivity(nodeID, 1)
	}
	i := 0
	utxoStatesProd := func() *chainstorage.OutputWithMetadata {
		if i == len(outputsWithMetadata) {
			return nil
		}

		o := outputsWithMetadata[i]
		i++

		return o
	}

	headerProd := func() (header *ledger.SnapshotHeader, err error) {
		header = &ledger.SnapshotHeader{
			FullEpochIndex: 0,
			DiffEpochIndex: 0,
			LatestECRecord: commitment.New(0, commitment.ID{}, types.Identifier{}, genesisTokenAmount),
		}

		return
	}

	sepsProd := func() (sep *snapshot.SolidEntryPoints) {
		return &snapshot.SolidEntryPoints{EI: epoch.Index(0), Seps: make([]models.BlockID, 0)}
	}

	epochDiffsProd := func() (diffs *ledger.EpochDiff) {
		outputs := make([]*chainstorage.OutputWithMetadata, 0)
		diffs = ledger.NewEpochDiff(outputs, outputs)
		return
	}

	activityLogProd := func() activitylog.SnapshotEpochActivity {
		return epochActivity
	}

	_, err = snapshot.CreateSnapshot(snapshotFileName, headerProd, sepsProd, utxoStatesProd, epochDiffsProd, activityLogProd)

	return
}

var outputCounter uint16 = 1

func createOutput(address devnetvm.Address, tokenAmount uint64, pledgeID identity.ID, creationTime time.Time) (output devnetvm.Output, outputMetadata *ledger.OutputMetadata) {
	output = devnetvm.NewSigLockedColoredOutput(devnetvm.NewColoredBalances(map[devnetvm.Color]uint64{
		devnetvm.ColorIOTA: tokenAmount,
	}), address)
	output.SetID(utxo.NewOutputID(utxo.EmptyTransactionID, outputCounter))
	outputCounter++

	outputMetadata = ledger.NewOutputMetadata(output.ID())
	outputMetadata.SetConfirmationState(confirmation.Confirmed)
	outputMetadata.SetAccessManaPledgeID(pledgeID)
	outputMetadata.SetConsensusManaPledgeID(pledgeID)
	outputMetadata.SetCreationTime(creationTime)

	return output, outputMetadata
}

*/
