package snapshotcreator

import (
	"time"

	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types/confirmation"

	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/dpos"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
)

// CreateSnapshot creates a new snapshot. Genesis is defined by genesisTokenAmount and seedBytes, it is pledged to the
// empty nodeID. The amount to pledge to each node is defined by nodesToPledge map, the funds of each pledge is burned.
// pledge funds
// | Pledge | Funds        |
// | ------ | ------------ |
// | empty  | genesisSeed  |
// | node1  | empty/burned |
// | node2  | empty/burned |
func CreateSnapshot(s *storage.Storage, snapshotFileName string, genesisTokenAmount uint64, genesisSeedBytes []byte, nodesToPledge map[ed25519.PublicKey]uint64) {
	now := time.Now()

	if err := s.Commitments.Store(0, &commitment.Commitment{}); err != nil {
		panic(err)
	}
	if err := s.Settings.SetChainID(lo.PanicOnErr(s.Commitments.Load(0)).ID()); err != nil {
		panic(err)
	}

	engineInstance := engine.New(s, dpos.NewProvider(), mana1.NewProvider())
	// prepare outputsWithMetadata
	output, outputMetadata := createOutput(seed.NewSeed(genesisSeedBytes).Address(0).Address(), genesisTokenAmount, identity.ID{}, now)

	if err := engineInstance.LedgerState.UnspentOutputs.ApplyCreatedOutput(ledgerstate.NewOutputWithMetadata(0, output.ID(), output, outputMetadata.ConsensusManaPledgeID(), outputMetadata.AccessManaPledgeID())); err != nil {
		panic(err)
	}

	for publicKey, value := range nodesToPledge {
		randomPublicKey := ed25519.GenerateKeyPair().PublicKey

		// pledge to ID but send funds to random address
		output, outputMetadata = createOutput(devnetvm.NewED25519Address(randomPublicKey), value, identity.NewID(publicKey), now)
		if err := engineInstance.LedgerState.UnspentOutputs.ApplyCreatedOutput(ledgerstate.NewOutputWithMetadata(0, output.ID(), output, outputMetadata.ConsensusManaPledgeID(), outputMetadata.AccessManaPledgeID())); err != nil {
			panic(err)
		}

		blk := models.NewBlock(models.WithStrongParents(models.NewBlockIDs(models.EmptyBlockID)), models.WithIssuingTime(now), models.WithIssuer(publicKey))
		if err := blk.DetermineID(); err != nil {
			panic(err)
		}

		if _, err := engineInstance.NotarizationManager.Attestations.Add(blk); err != nil {
			panic(err)
		}
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
// | genesisSeed | genesisSeed |
// | node1       | node1       |
// | node2       | node2       |
func CreateSnapshotForIntegrationTest(s *storage.Storage, snapshotFileName string, genesisTokenAmount uint64, genesisSeedBytes []byte, genesisNodePledge []byte, nodesToPledge map[[32]byte]uint64) {
	now := time.Now()
	engineInstance := engine.New(s, dpos.NewProvider(), mana1.NewProvider())

	// This is the same seed used to derive the faucet ID.
	genesisPrivateKey := ed25519.PrivateKeyFromSeed(genesisNodePledge)
	genesisPublicKey := genesisPrivateKey.Public()
	genesisPledgeID := identity.New(genesisPublicKey).ID()
	output, outputMetadata := createOutput(seed.NewSeed(genesisSeedBytes).Address(0).Address(), genesisTokenAmount, genesisPledgeID, now)
	if err := engineInstance.LedgerState.UnspentOutputs.ApplyCreatedOutput(ledgerstate.NewOutputWithMetadata(0, output.ID(), output, outputMetadata.ConsensusManaPledgeID(), outputMetadata.AccessManaPledgeID())); err != nil {
		panic(err)
	} else if _, err := engineInstance.NotarizationManager.Attestations.Add(models.NewBlock(
		models.WithIssuingTime(now),
		models.WithIssuer(genesisPublicKey),
	)); err != nil {
		panic(err)
	}

	for nodeSeedBytes, value := range nodesToPledge {
		privateKey := ed25519.PrivateKeyFromSeed(nodeSeedBytes[:])
		publicKey := privateKey.Public()
		nodeID := identity.New(publicKey).ID()
		output, outputMetadata = createOutput(seed.NewSeed(nodeSeedBytes[:]).Address(0).Address(), value, nodeID, now)
		if err := engineInstance.LedgerState.UnspentOutputs.ApplyCreatedOutput(ledgerstate.NewOutputWithMetadata(0, output.ID(), output, outputMetadata.ConsensusManaPledgeID(), outputMetadata.AccessManaPledgeID())); err != nil {
			panic(err)
		}

		blk := models.NewBlock(models.WithStrongParents(models.NewBlockIDs(models.EmptyBlockID)), models.WithIssuingTime(now), models.WithIssuer(publicKey))
		blk.DetermineID()
		if err := blk.DetermineID(); err != nil {
			panic(err)
		}
		if _, err := engineInstance.NotarizationManager.Attestations.Add(blk); err != nil {
			panic(err)
		}
	}

	if err := engineInstance.WriteSnapshot(snapshotFileName); err != nil {
		panic(err)
	}
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
