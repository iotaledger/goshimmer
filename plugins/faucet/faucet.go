package faucet

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/app/faucet"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm/indexer"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/goshimmer/packages/node/clock"
	"github.com/iotaledger/goshimmer/plugins/blocklayer"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/queue"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types"
)

var (
	maxTxBookedAwaitTime = 5 * time.Second
	waitForAcceptance    = 10 * time.Second
	maxWaitAttempts      = 5

	addrIndexMap = make(map[string]uint64, 0)
)

// remainder stays on index 0
type Faucet struct {
	seed             *seed.Seed
	remainderOutput  *faucetOutput
	availableOutputs *queue.Queue[*faucetOutput]
}

// NewFaucet creates a new Faucet instance.
func NewFaucet(seed *seed.Seed) *Faucet {
	f := &Faucet{
		seed:             seed,
		availableOutputs: queue.New[*faucetOutput](Parameters.SupplyOutputsCount),
	}

	for i := uint64(0); i <= uint64(Parameters.SupplyOutputsCount); i++ {
		addrIndexMap[f.seed.Address(i).Address().Base58()] = i
	}

	return f
}

// Start starts the faucet to fulfill faucet requests.
func (f *Faucet) Start(ctx context.Context, requestChan <-chan *faucet.Payload) {
	for {
		select {
		case p := <-requestChan:
			blk, err := f.handleFaucetRequest(p)
			if err != nil {
				Plugin.LogErrorf("fail to send funds to %s: %v", p.Address().Base58(), err)
				return
			}
			Plugin.LogInfof("send funds to %s: blockID: %s", p.Address().Base58(), blk.ID().Base58())

		case <-ctx.Done():
			return
		}
	}
}

// DeriveStateFromTangle derives unspent outputs of faucet from snapshot.
func (f *Faucet) DeriveStateFromTangle() {
	supplyFound := 0
	var remainderAmount uint64

	for i := uint64(0); i <= uint64(Parameters.SupplyOutputsCount); i++ {
		deps.Indexer.CachedAddressOutputMappings(f.seed.Address(i).Address()).Consume(func(mapping *indexer.AddressOutputMapping) {
			deps.Tangle.Ledger.Storage.CachedOutput(mapping.OutputID()).Consume(func(output utxo.Output) {
				deps.Tangle.Ledger.Storage.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *ledger.OutputMetadata) {
					if !outputMetadata.IsSpent() {
						outputEssence := output.(devnetvm.Output)

						iotaBalance, colorExist := outputEssence.Balances().Get(devnetvm.ColorIOTA)
						if !colorExist {
							return
						}
						o := &faucetOutput{
							ID:           outputEssence.ID(),
							Address:      outputEssence.Address(),
							AddressIndex: i,
							Balance:      iotaBalance,
						}

						if i == 0 {
							f.remainderOutput = o
							remainderAmount = iotaBalance
						} else {
							// we found a prepared output
							f.availableOutputs.Offer(o)
							supplyFound++
						}
					}
				})
			})
		})
	}

	// split funds in advance if no supply output is found
	if supplyFound == 0 {
		f.prepareUnspentOutputs()
	}

	Plugin.LogInfof("Faucet has remainder with %d IOTA", remainderAmount)
	Plugin.LogInfof("Faucet has %d supply output prepared", supplyFound)
}

func (f *Faucet) handleFaucetRequest(p *faucet.Payload) (*tangleold.Block, error) {
	// get an unspent output
	o, err := f.getUnspentOutput()
	if err != nil {
		return nil, err
	}

	// prepare tx
	tx := f.prepareFaucetTransaction(p.Address(), o, p.AccessManaPledgeID(), p.ConsensusManaPledgeID())
	blk, err := issueTx(tx)
	if err != nil {
		return nil, err
	}

	return blk, nil
}

// prepareFaucetTransaction prepares a funding faucet transaction that spends fundingOutput to destAddr and pledges
// mana to pledgeID.
func (f *Faucet) prepareFaucetTransaction(destAddr devnetvm.Address, fundingOutput *faucetOutput, accessManaPledgeID, consensusManaPledgeID identity.ID) (tx *devnetvm.Transaction) {
	inputs := devnetvm.NewInputs(devnetvm.NewUTXOInput(fundingOutput.ID))
	outputs := devnetvm.NewOutputs(devnetvm.NewSigLockedColoredOutput(
		devnetvm.NewColoredBalances(
			map[devnetvm.Color]uint64{
				devnetvm.ColorIOTA: uint64(Parameters.TokensPerRequest),
			}),
		destAddr,
	))

	w := wallet{keyPair: *f.seed.KeyPair(fundingOutput.AddressIndex)}
	tx = makeTransaction(inputs, outputs, w)

	return
}

func (f *Faucet) getUnspentOutput() (*faucetOutput, error) {
	// split funds if no unspent output is in the queue
	if f.availableOutputs.Size() == 0 {
		if err := f.prepareUnspentOutputs(); err != nil {
			return nil, err
		}
	}

	o, _ := f.availableOutputs.Poll()
	return o, nil
}

func (f *Faucet) prepareUnspentOutputs() error {
	Plugin.LogInfof("Start splitting funds to %d addresses", Parameters.SupplyOutputsCount)

	errChan := make(chan error)
	listenerAttachedChan := make(chan types.Empty)
	finished := make(chan types.Empty)

	inputs := devnetvm.NewInputs(devnetvm.NewUTXOInput(f.remainderOutput.ID))
	outputs := make(devnetvm.Outputs, 0)

	// split outputs to Parameters.SupplyOutputsCount addresses
	for i := uint64(1); i <= uint64(Parameters.SupplyOutputsCount); i++ {
		addr := f.seed.Address(i).Address()
		outputs = append(outputs, devnetvm.NewSigLockedColoredOutput(
			devnetvm.NewColoredBalances(
				map[devnetvm.Color]uint64{
					devnetvm.ColorIOTA: uint64(Parameters.TokensPerRequest),
				}),
			addr,
		))
	}

	// append remainder output, remainder is always on address index 0
	outputs = append(outputs, devnetvm.NewSigLockedColoredOutput(
		devnetvm.NewColoredBalances(
			map[devnetvm.Color]uint64{
				devnetvm.ColorIOTA: f.remainderOutput.Balance - uint64(Parameters.TokensPerRequest)*uint64(Parameters.SupplyOutputsCount),
			}),
		f.remainderOutput.Address,
	))

	w := wallet{keyPair: *f.seed.KeyPair(f.remainderOutput.AddressIndex)}
	tx := makeTransaction(inputs, outputs, w)

	go awaitTransactionToConfirmed(tx.ID(), errChan, listenerAttachedChan, finished)
	<-listenerAttachedChan

	if _, err := issueTx(tx); err != nil {
		return err
	}

	for {
		select {
		case <-finished:
			// update remainder output and available outputs
			outputMap := tx.Essence().Outputs().ByID()
			for id, o := range outputMap {
				b, _ := o.Balances().Get(devnetvm.ColorIOTA)
				fOutput := &faucetOutput{
					ID:           id,
					Address:      o.Address(),
					AddressIndex: addrIndexMap[o.Address().Base58()],
					Balance:      b,
				}

				if fOutput.AddressIndex == 0 {
					f.remainderOutput = fOutput
				} else {
					f.availableOutputs.Offer(fOutput)
				}
			}

			return nil
		case err := <-errChan:
			return err
		}
	}
}

// awaitTransactionToConfirmed listens for the confirmation.
// Listening is finished when issued transactions is confirmed or when the awaiting time is up.
func awaitTransactionToConfirmed(txID utxo.TransactionID, preparationFailure chan error, listenerAttached, finished chan<- types.Empty) {
	Plugin.LogInfof("Start listening for confirmation")
	txConfirmed := make(chan struct{})

	monitorTxAcceptance := event.NewClosure(func(event *ledger.TransactionAcceptedEvent) {
		if txID == event.TransactionID {
			close(txConfirmed)
		}
	})

	// listen on confirmation
	deps.Tangle.Ledger.Events.TransactionAccepted.Attach(monitorTxAcceptance)
	defer deps.Tangle.Ledger.Events.TransactionAccepted.Detach(monitorTxAcceptance)

	ticker := time.NewTicker(waitForAcceptance)
	defer ticker.Stop()

	close(listenerAttached)
	attempt := 0

	// waiting for transactions to be confirmed
	for {
		select {
		case <-txConfirmed:
			close(finished)
			return
		case <-ticker.C:
			accepted := onTickerCheckConfirmation(txID)
			if accepted {
				close(finished)
				return
			}
			if attempt > maxWaitAttempts {
				preparationFailure <- errors.Errorf("splitted transaction from faucet is not confirmed in time")
				return
			}
			attempt++
		}
	}
}

func onTickerCheckConfirmation(expectedTXID utxo.TransactionID) bool {
	found := false
	deps.Tangle.Ledger.Storage.CachedTransactionMetadata(expectedTXID).Consume(func(t *ledger.TransactionMetadata) {
		if t.ConfirmationState().IsAccepted() {
			found = true
		}
	})
	return found
}

func makeTransaction(inputs devnetvm.Inputs, outputs devnetvm.Outputs, w wallet) (tx *devnetvm.Transaction) {
	essence := devnetvm.NewTransactionEssence(
		0,
		clock.SyncedTime(),
		deps.Local.ID(),
		identity.ID{},
		devnetvm.NewInputs(inputs...),
		devnetvm.NewOutputs(outputs...),
	)

	unlockBlock := devnetvm.NewSignatureUnlockBlock(w.sign(essence))

	tx = devnetvm.NewTransaction(
		essence,
		devnetvm.UnlockBlocks{unlockBlock},
	)

	return
}

// issueTx issues a transaction to the Tangle and waits for it to become booked.
func issueTx(tx *devnetvm.Transaction) (blk *tangleold.Block, err error) {
	// attach to block layer
	issueTransaction := func() (*tangleold.Block, error) {
		block, e := deps.Tangle.IssuePayload(tx)
		if e != nil {
			return nil, e
		}
		return block, nil
	}

	// block for a certain amount of time until we know that the transaction
	// actually got booked by this node itself
	// TODO: replace with an actual more reactive way
	blk, err = blocklayer.AwaitBlockToBeBooked(issueTransaction, tx.ID(), maxTxBookedAwaitTime)
	if err != nil {
		return nil, errors.Errorf("%w: tx %s", err, tx.ID().String())
	}
	return blk, nil
}

type wallet struct {
	keyPair ed25519.KeyPair
}

func (w wallet) privateKey() ed25519.PrivateKey {
	return w.keyPair.PrivateKey
}

func (w wallet) publicKey() ed25519.PublicKey {
	return w.keyPair.PublicKey
}

func (w wallet) sign(txEssence *devnetvm.TransactionEssence) *devnetvm.ED25519Signature {
	return devnetvm.NewED25519Signature(w.publicKey(), w.privateKey().Sign(lo.PanicOnErr(txEssence.Bytes())))
}

type faucetOutput struct {
	ID           utxo.OutputID
	Balance      uint64
	Address      devnetvm.Address
	AddressIndex uint64
}
