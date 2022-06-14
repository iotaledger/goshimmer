package snapshotcreator

import (
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/generics/orderedmap"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/snapshot"
)

// CreateSnapshot creates a new snapshot. Genesis is defined by genesisTokenAmount and seedBytes, it is pledged to the
// empty nodeID. The amount to pledge to each node is defined by nodesToPledge map, the funds of each pledge is burned.
// pledge funds
// | Pledge | Funds        |
// | ------ | ------------ |
// | empty  | genesisSeed  |
// | node1  | empty/burned |
// | node2  | empty/burned |
func CreateSnapshot(genesisTokenAmount uint64, genesisSeedBytes []byte, nodesToPledge map[identity.ID]uint64) (createdSnapshot *snapshot.Snapshot, err error) {
	now := time.Now()
	outputs := utxo.NewOutputs()
	outputsMetadata := ledger.NewOutputsMetadata()

	output, outputMetadata := createOutput(seed.NewSeed(genesisSeedBytes).Address(0).Address(), genesisTokenAmount, identity.ID{}, now)
	outputs.Add(output)
	outputsMetadata.Add(outputMetadata)

	for nodeID, value := range nodesToPledge {
		// pledge to empty ID (burn tokens)
		output, outputMetadata = createOutput(devnetvm.NewED25519Address(ed25519.GenerateKeyPair().PublicKey), value, nodeID, now)
		outputs.Add(output)
		outputsMetadata.Add(outputMetadata)
	}

	ledgerSnapshot := ledger.NewSnapshot(outputs, outputsMetadata)
	ledgerSnapshot.FullEpochIndex = 0
	ledgerSnapshot.DiffEpochIndex = 0
	ledgerSnapshot.EpochDiffs = &ledger.EpochDiffs{*orderedmap.New[epoch.Index, *ledger.EpochDiff]()}
	ledgerSnapshot.LatestECRecord = epoch.NewECRecord(0)
	ledgerSnapshot.LatestECRecord.SetECR(&epoch.MerkleRoot{types.NewIdentifier([]byte{})})
	ledgerSnapshot.LatestECRecord.SetPrevEC(&epoch.MerkleRoot{types.NewIdentifier([]byte{})})

	return &snapshot.Snapshot{
		LedgerSnapshot: ledgerSnapshot,
	}, nil
}

// CreateSnapshotForIntegrationTest creates a new snapshot. Genesis is defined by genesisTokenAmount and seedBytes, it
// is pledged to the node that is derived from the same seed. The amount to pledge to each node is defined by
// nodesToPledge map (seedBytes->amount), the funds of each pledge is sent to the same seed.
// | Pledge      | Funds       |
// | ----------- | ----------- |
// | genesisSeed | genesisSeed |
// | node1       | node1       |
// | node2       | node2       |
func CreateSnapshotForIntegrationTest(genesisTokenAmount uint64, seedBytes []byte, genesisNodePledge []byte, nodesToPledge map[[32]byte]uint64) (createdSnapshot *snapshot.Snapshot, err error) {
	now := time.Now()
	outputs := utxo.NewOutputs()
	outputsMetadata := ledger.NewOutputsMetadata()
	manaSnapshot := mana.NewSnapshot()

	genesisIdentity := identity.New(ed25519.PrivateKeyFromSeed(genesisNodePledge).Public()).ID()
	output, outputMetadata := createOutput(seed.NewSeed(seedBytes).Address(0).Address(), genesisTokenAmount, genesisIdentity, now)
	outputs.Add(output)
	outputsMetadata.Add(outputMetadata)

	manaSnapshot.ByNodeID[genesisIdentity] = &mana.SnapshotNode{
		AccessMana: &mana.AccessManaSnapshot{
			Value:     float64(genesisTokenAmount),
			Timestamp: now,
		},
		SortedTxSnapshot: mana.SortedTxSnapshot{
			&mana.TxSnapshot{
				TxID:      output.ID().TransactionID,
				Timestamp: now,
				Value:     float64(genesisTokenAmount),
			},
		},
	}

	for nodeSeedBytes, value := range nodesToPledge {
		// pledge to empty ID (burn tokens)
		output, outputMetadata = createOutput(seed.NewSeed(nodeSeedBytes[:]).Address(0).Address(), value, nodeSeedBytes, now)
		outputs.Add(output)
		outputsMetadata.Add(outputMetadata)

		manaSnapshot.ByNodeID[identity.New(ed25519.PrivateKeyFromSeed(nodeSeedBytes[:]).Public()).ID()] = &mana.SnapshotNode{
			AccessMana: &mana.AccessManaSnapshot{
				Value:     float64(value),
				Timestamp: now,
			},
			SortedTxSnapshot: mana.SortedTxSnapshot{
				&mana.TxSnapshot{
					TxID:      output.ID().TransactionID,
					Timestamp: now,
					Value:     float64(value),
				},
			},
		}
	}

	return &snapshot.Snapshot{
		LedgerSnapshot: ledger.NewSnapshot(outputs, outputsMetadata),
	}, nil
}

var outputCounter uint16 = 1

func createOutput(address devnetvm.Address, tokenAmount uint64, pledgeID identity.ID, creationTime time.Time) (output devnetvm.Output, outputMetadata *ledger.OutputMetadata) {
	output = devnetvm.NewSigLockedColoredOutput(devnetvm.NewColoredBalances(map[devnetvm.Color]uint64{
		devnetvm.ColorIOTA: tokenAmount,
	}), address)
	output.SetID(utxo.NewOutputID(utxo.EmptyTransactionID, outputCounter))
	outputCounter++

	outputMetadata = ledger.NewOutputMetadata(output.ID())
	outputMetadata.SetGradeOfFinality(gof.High)
	outputMetadata.SetConsensusManaPledgeID(pledgeID)
	outputMetadata.SetAccessManaPledgeID(pledgeID)
	outputMetadata.SetCreationTime(creationTime)

	return output, outputMetadata
}
