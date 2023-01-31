package snapshotcreator

import (
	"os"
	"time"

	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/orderedmap"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types/confirmation"

	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/dpos"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/storage"
)

// CreateSnapshot creates a new snapshot. Genesis is defined by genesisTokenAmount and seedBytes, it is pledged to the
// empty nodeID. The amount to pledge to each node is defined by nodesToPledge map, the funds of each pledge is burned.
// pledge funds
// | Pledge | Funds        |
// | ------ | ------------ |
// | empty  | genesisSeed  |
// | node1  | node1		   |
// | node2  | node2        |.
func CreateSnapshot(databaseVersion database.Version, snapshotFileName string, genesisTokenAmount uint64, genesisSeedBytes []byte, nodesToPledge map[ed25519.PublicKey]uint64, initialAttestations []ed25519.PublicKey, engineOpts ...options.Option[engine.Engine]) {
	s := storage.New(lo.PanicOnErr(os.MkdirTemp(os.TempDir(), "*")), databaseVersion)

	if err := s.Commitments.Store(commitment.NewEmptyCommitment()); err != nil {
		panic(err)
	}
	if err := s.Settings.SetChainID(lo.PanicOnErr(s.Commitments.Load(0)).ID()); err != nil {
		panic(err)
	}

	engineInstance := engine.New(s, dpos.NewProvider(), mana1.NewProvider(), engineOpts...)

	// Create genesis output
	if genesisTokenAmount > 0 {
		output, outputMetadata := createOutput(seed.NewSeed(genesisSeedBytes).Address(0).Address(), genesisTokenAmount, identity.ID{}, 0)
		if err := engineInstance.LedgerState.UnspentOutputs.ApplyCreatedOutput(ledger.NewOutputWithMetadata(0, output.ID(), output, outputMetadata.ConsensusManaPledgeID(), outputMetadata.AccessManaPledgeID())); err != nil {
			panic(err)
		}
	}

	// Create outputs for nodes
	engineInstance.NotarizationManager.Attestations.SetLastCommittedEpoch(-1)
	for nodePublicKey, value := range nodesToPledge {
		// pledge to ID but send funds to random address
		nodeID := identity.NewID(nodePublicKey)
		output, outputMetadata := createOutput(devnetvm.NewED25519Address(nodePublicKey), value, nodeID, 0)
		if err := engineInstance.LedgerState.UnspentOutputs.ApplyCreatedOutput(ledger.NewOutputWithMetadata(0, output.ID(), output, outputMetadata.ConsensusManaPledgeID(), outputMetadata.AccessManaPledgeID())); err != nil {
			panic(err)
		}
	}

	for _, nodeID := range initialAttestations {
		if _, err := engineInstance.NotarizationManager.Attestations.Add(&notarization.Attestation{
			IssuerPublicKey: nodeID,
			IssuingTime:     time.Unix(epoch.GenesisTime-1, 0),
		}); err != nil {
			panic(err)
		}
	}

	if _, _, err := engineInstance.NotarizationManager.Attestations.Commit(0); err != nil {
		panic(err)
	}

	if err := engineInstance.WriteSnapshot(snapshotFileName); err != nil {
		panic(err)
	}
}

// CreateSnapshotForIntegrationTest creates a new snapshot. Genesis is defined by genesisTokenAmount and seedBytes, it
// is pledged to the node that is derived from the same seed. The amount to pledge to each node is defined by
// nodesToPledge map (seedBytes->amount), the funds of each pledge is sent to the same seed.
// | Pledge      | Funds       |
// | ----------- | ----------- |
// | empty       | genesisSeed  |
// | node1       | node1       |
// | node2       | node2       |.
func CreateSnapshotForIntegrationTest(s *storage.Storage, snapshotFileName string, genesisTokenAmount uint64, genesisSeedBytes []byte, nodesToPledge *orderedmap.OrderedMap[identity.ID, uint64], startSynced bool, engineOpts ...options.Option[engine.Engine]) {
	if err := s.Commitments.Store(commitment.NewEmptyCommitment()); err != nil {
		panic(err)
	}
	if err := s.Settings.SetChainID(lo.PanicOnErr(s.Commitments.Load(0)).ID()); err != nil {
		panic(err)
	}

	engineInstance := engine.New(s, dpos.NewProvider(), mana1.NewProvider(), engineOpts...)

	engineInstance.NotarizationManager.Attestations.SetLastCommittedEpoch(-1)

	if genesisTokenAmount > 0 {
		// create faucet funds and do not pledge mana to any identity
		var genesisPledgeID identity.ID
		output, outputMetadata := createOutput(seed.NewSeed(genesisSeedBytes).Address(0).Address(), genesisTokenAmount, genesisPledgeID, 0)
		if err := engineInstance.LedgerState.UnspentOutputs.ApplyCreatedOutput(ledger.NewOutputWithMetadata(0, output.ID(), output, outputMetadata.ConsensusManaPledgeID(), outputMetadata.AccessManaPledgeID())); err != nil {
			panic(err)
		}
	}

	i := 0
	nodesToPledge.ForEach(func(nodeSeedBytes identity.ID, value uint64) bool {
		nodePublicKey := ed25519.PrivateKeyFromSeed(nodeSeedBytes[:]).Public()
		nodeID := identity.NewID(nodePublicKey)
		output, outputMetadata := createOutput(seed.NewSeed(nodeSeedBytes[:]).Address(0).Address(), value, nodeID, 0)
		if err := engineInstance.LedgerState.UnspentOutputs.ApplyCreatedOutput(ledger.NewOutputWithMetadata(0, output.ID(), output, outputMetadata.ConsensusManaPledgeID(), outputMetadata.AccessManaPledgeID())); err != nil {
			panic(err)
		}

		if i == 0 || startSynced {
			// Add attestation to commitment only for first peer, so that it can issue blocks and bootstraps the network.
			if _, err := engineInstance.NotarizationManager.Attestations.Add(&notarization.Attestation{
				IssuerPublicKey: nodePublicKey,
				IssuingTime:     time.Unix(epoch.GenesisTime-1, 0),
			}); err != nil {
				panic(err)
			}
		}

		i++
		return true
	})
	if _, _, err := engineInstance.NotarizationManager.Attestations.Commit(0); err != nil {
		panic(err)
	}

	if err := engineInstance.WriteSnapshot(snapshotFileName); err != nil {
		panic(err)
	}
}

var outputCounter uint16 = 1

func createOutput(address devnetvm.Address, tokenAmount uint64, pledgeID identity.ID, includedInEpoch epoch.Index) (output devnetvm.Output, outputMetadata *ledger.OutputMetadata) {
	output = devnetvm.NewSigLockedColoredOutput(devnetvm.NewColoredBalances(map[devnetvm.Color]uint64{
		devnetvm.ColorIOTA: tokenAmount,
	}), address)
	output.SetID(utxo.NewOutputID(utxo.EmptyTransactionID, outputCounter))
	outputCounter++

	outputMetadata = ledger.NewOutputMetadata(output.ID())
	outputMetadata.SetConfirmationState(confirmation.Confirmed)
	outputMetadata.SetAccessManaPledgeID(pledgeID)
	outputMetadata.SetConsensusManaPledgeID(pledgeID)
	outputMetadata.SetInclusionEpoch(includedInEpoch)

	return output, outputMetadata
}
