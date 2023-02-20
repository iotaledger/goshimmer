package snapshotcreator

import (
	"os"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/dpos"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/orderedmap"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/workerpool"
)

// CreateSnapshot creates a new snapshot. Genesis is defined by genesisTokenAmount and seedBytes, it
// is pledged to the node that is derived from the same seed. The amount to pledge to each node is defined by
// nodesToPledge map (seedBytes->amount), the funds of each pledge is sent to the same seed.
// | Pledge      | Funds       |
// | ----------- | ----------- |
// | empty       | genesisSeed  |
// | node1       | node1       |
// | node2       | node2       |.

func CreateSnapshot(opts ...options.Option[Options]) (err error) {
	opt := NewOptions(opts...)

	workers := workerpool.NewGroup("CreateSnapshot")
	defer workers.Shutdown()
	s := storage.New(lo.PanicOnErr(os.MkdirTemp(os.TempDir(), "*")), opt.dataBaseVersion)
	defer s.Shutdown()

	if err = s.Commitments.Store(commitment.NewEmptyCommitment()); err != nil {
		return errors.Wrap(err, "failed to store empty commitment")
	}
	if err = s.Settings.SetChainID(lo.PanicOnErr(s.Commitments.Load(0)).ID()); err != nil {
		return errors.Wrap(err, "failed to set chainID")
	}

	engineInstance := engine.New(workers.CreateGroup("Engine"), s, dpos.NewProvider(), mana1.NewProvider(), engine.WithLedgerOptions(ledger.WithVM(opt.vm)))
	defer engineInstance.Shutdown()

	err = opt.createGenesisOutput(engineInstance)
	if err != nil {
		return err
	}
	engineInstance.NotarizationManager.Attestations.SetLastCommittedEpoch(-1)

	i := 0
	nodesToPledge, err := opt.createPledgeMap()
	if err != nil {
		return err
	}

	nodesToPledge.ForEach(func(nodeIdentity *identity.Identity, value uint64) bool {
		nodePublicKey := nodeIdentity.PublicKey()
		nodeID := nodeIdentity.ID()
		output, outputMetadata, errOut := createOutput(opt.vm, nodePublicKey, value, nodeID, 0)
		if errOut != nil {
			panic(errOut)
		}
		if err = engineInstance.LedgerState.UnspentOutputs.ApplyCreatedOutput(ledger.NewOutputWithMetadata(0, output.ID(), output, outputMetadata.ConsensusManaPledgeID(), outputMetadata.AccessManaPledgeID())); err != nil {
			panic(err)
		}

		// Add attestation to commitment only if AttestAll option is set.
		if opt.AttestAll {
			err = opt.attest(engineInstance, nodePublicKey)
		}
		i++
		return true
	})

	err = opt.createAttestationIfNotYetDone(engineInstance)
	if err != nil {
		return err
	}
	if _, _, err = engineInstance.NotarizationManager.Attestations.Commit(0); err != nil {
		return err
	}
	if err := engineInstance.WriteSnapshot(opt.FilePath); err != nil {
		return err
	}
	return nil
}

func (m *Options) attest(engineInstance *engine.Engine, nodePublicKey ed25519.PublicKey) error {
	if _, err := engineInstance.NotarizationManager.Attestations.Add(&notarization.Attestation{
		IssuerPublicKey: nodePublicKey,
		IssuingTime:     time.Unix(epoch.GenesisTime-1, 0),
	}); err != nil {
		return err
	}
	return nil
}

func (m *Options) createAttestationIfNotYetDone(engineInstance *engine.Engine) (err error) {
	if m.AttestAll {
		return nil
	}
	for _, nodePublicKey := range m.InitialAttestationsPublicKey {
		err = m.attest(engineInstance, nodePublicKey)
		if err != nil {
			return errors.Wrap(err, "failed to attest")
		}
	}
	return nil
}

func (m *Options) createGenesisOutput(engineInstance *engine.Engine) error {
	if m.GenesisTokenAmount > 0 {
		output, outputMetadata, err := createOutput(m.vm, seed.NewSeed(m.GenesisSeed).KeyPair(0).PublicKey, m.GenesisTokenAmount, identity.ID{}, 0)
		if err != nil {
			return err
		}
		if err := engineInstance.LedgerState.UnspentOutputs.ApplyCreatedOutput(ledger.NewOutputWithMetadata(0, output.ID(), output, outputMetadata.ConsensusManaPledgeID(), outputMetadata.AccessManaPledgeID())); err != nil {
			return err
		}
	}
	return nil
}

// createPledgeMap creates a pledge map according to snapshotInfo
func (m *Options) createPledgeMap() (nodesToPledge *orderedmap.OrderedMap[*identity.Identity, uint64], err error) {
	nodesToPledge = orderedmap.New[*identity.Identity, uint64]()
	if m.PeersAmountsPledged == nil {
		m.PeersAmountsPledged = make([]uint64, len(m.PeersPublicKey))
		// equal snapshot by default
		if m.TotalTokensPledged == 0 {
			m.TotalTokensPledged = m.GenesisTokenAmount
		}
		for i := range m.PeersAmountsPledged {
			m.PeersAmountsPledged[i] = m.TotalTokensPledged / uint64(len(m.PeersPublicKey))
		}
	}

	for i, pubKey := range m.PeersPublicKey {
		nodesToPledge.Set(identity.New(pubKey), m.PeersAmountsPledged[i])
	}

	return nodesToPledge, nil
}

var outputCounter uint16 = 1

func createOutput(ledgerVM vm.VM, publicKey ed25519.PublicKey, tokenAmount uint64, pledgeID identity.ID, includedInEpoch epoch.Index) (output utxo.Output, outputMetadata *ledger.OutputMetadata, err error) {
	switch ledgerVM.(type) {
	case *ledger.MockedVM:
		output = ledger.NewMockedOutput(utxo.EmptyTransactionID, outputCounter, tokenAmount)

	case *devnetvm.VM:
		output = devnetvm.NewSigLockedColoredOutput(devnetvm.NewColoredBalances(map[devnetvm.Color]uint64{
			devnetvm.ColorIOTA: tokenAmount,
		}), devnetvm.NewED25519Address(publicKey))
		output.SetID(utxo.NewOutputID(utxo.EmptyTransactionID, outputCounter))

	default:
		return nil, nil, errors.Errorf("cannot create snapshot output for VM of type '%v'", ledgerVM)
	}

	outputCounter++

	outputMetadata = ledger.NewOutputMetadata(output.ID())
	outputMetadata.SetConfirmationState(confirmation.Confirmed)
	outputMetadata.SetAccessManaPledgeID(pledgeID)
	outputMetadata.SetConsensusManaPledgeID(pledgeID)
	outputMetadata.SetInclusionEpoch(includedInEpoch)

	return output, outputMetadata, nil
}
