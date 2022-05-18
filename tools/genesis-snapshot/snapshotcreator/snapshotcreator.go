package snapshotcreator

import (
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/snapshot"
)

// CreateSnapshot writes a new snapshot file to the path declared by snapshot name. Genesis is defined by genesisTokenAmount
// and seedBytes. The amount pledge to each node is defined by nodesToPledge map. Whenever the amount is 0 in the map pledgeTokenAmount is used.
func CreateSnapshot(genesisTokenAmount uint64, seedBytes []byte, nodesToPledge map[identity.ID]uint64) (createdSnapshot *snapshot.Snapshot, err error) {
	now := time.Now()
	genesisSeed := seed.NewSeed(seedBytes)
	outputs := utxo.NewOutputs()
	outputsMetadata := ledger.NewOutputsMetadata()

	output, outputMetadata := createOutput(genesisSeed.Address(0).Address(), genesisTokenAmount, identity.ID{}, now)
	outputs.Add(output)
	outputsMetadata.Add(outputMetadata)

	manaSnapshot := mana.NewSnapshot()
	for nodeID, value := range nodesToPledge {
		// pledge to empty ID (burn tokens)
		output, outputMetadata = createOutput(devnetvm.NewED25519Address(ed25519.GenerateKeyPair().PublicKey), value, nodeID, now)
		outputs.Add(output)
		outputsMetadata.Add(outputMetadata)

		manaSnapshot.ByNodeID[nodeID] = &mana.SnapshotNode{
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
		ManaSnapshot:   manaSnapshot,
	}, nil
}

var outputCounter uint16

func createOutput(address devnetvm.Address, tokenAmount uint64, pledgeID identity.ID, creationTime time.Time) (output devnetvm.Output, outputMetadata *ledger.OutputMetadata) {
	output = devnetvm.NewSigLockedColoredOutput(devnetvm.NewColoredBalances(map[devnetvm.Color]uint64{
		devnetvm.ColorIOTA: tokenAmount,
	}), address)
	output.SetID(utxo.NewOutputID(utxo.EmptyTransactionID, outputCounter))
	outputCounter++

	outputMetadata = ledger.NewOutputMetadata(output.ID())
	outputMetadata.SetGradeOfFinality(gof.High)
	outputMetadata.SetConsensusManaPledgeID(pledgeID)
	outputMetadata.SetCreationTime(creationTime)
	outputMetadata.SetBranchIDs(set.NewAdvancedSet(utxo.EmptyTransactionID))

	return output, outputMetadata
}
