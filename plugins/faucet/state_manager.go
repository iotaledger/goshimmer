package faucet

import (
	"container/list"
	"context"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types"
	"github.com/iotaledger/hive.go/core/typeutils"
	"github.com/iotaledger/hive.go/core/workerpool"
	"go.uber.org/atomic"

	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/app/faucet"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm/indexer"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/goshimmer/packages/node/clock"
	"github.com/iotaledger/goshimmer/plugins/blocklayer"
)

const (
	// RemainderAddressIndex is the RemainderAddressIndex.
	RemainderAddressIndex = 0

	// MinFundingOutputsPercentage defines the min percentage of prepared funding outputs left that triggers a replenishment.
	MinFundingOutputsPercentage = 0.3

	// MaxFaucetOutputsCount defines the max outputs count for the Faucet as the ledgerstate.MaxOutputCount -1 remainder output.
	MaxFaucetOutputsCount = devnetvm.MaxOutputCount - 1

	// WaitForAcceptance defines the wait time before considering a transaction confirmed.
	WaitForAcceptance = 10 * time.Second

	// MaxWaitAttempts defines the number of attempts taken while waiting for confirmation during funds preparation.
	MaxWaitAttempts = 50

	// minFaucetBalanceMultiplier defines the multiplier for the min token amount required, before the faucet stops operating.
	minFaucetBalanceMultiplier = 0.1
)

// FaucetOutput represents an output controlled by the faucet.
type FaucetOutput struct {
	ID           utxo.OutputID
	Balance      uint64
	Address      devnetvm.Address
	AddressIndex uint64
}

// StateManager manages the funds and outputs of the faucet. Can derive its state from a synchronized Tangle, can
// carry out funding requests, and prepares more funding outputs when needed.
type StateManager struct {
	// the amount of tokens to send to every request
	tokensPerRequest uint64
	// number of supply outputs to generate per replenishment that will be split further into funding outputs
	targetSupplyOutputsCount uint64
	// number of funding outputs to generate per batch
	targetFundingOutputsCount uint64
	// the threshold of remaining available funding outputs under which the faucet starts to replenish new funding outputs
	replenishThreshold float64
	// number of funding outputs to generate per supply output in a supply transaction during the splitting period
	splittingMultiplier uint64
	// the number of tokens on each supply output
	tokensPerSupplyOutput uint64
	// the amount of tokens a supply replenishment will deduct from the faucet remainder
	tokensUsedOnSupplyReplenishment uint64

	// the time to await for the transaction fulfilling a funding request
	// to become booked in the value layer
	maxTxBookedAwaitTime time.Duration

	// fundingState serves fundingOutputs and its mutex
	fundingState *fundingState

	// replenishmentState keeps all variables and related methods used to track faucet state during funds replenishment
	replenishmentState *replenishmentState

	// splittingEnv keeps all variables and related methods necessary to split transactions during funds replenishment
	splittingEnv *splittingEnv

	// signal received from Faucet background worker on shutdown
	shutdownSignal <-chan struct{}
}

// NewStateManager creates a new state manager for the faucet.
func NewStateManager(
	tokensPerRequest uint64,
	seed *walletseed.Seed,
	supplyOutputsCount uint64,
	splittingMultiplier uint64,
	maxTxBookedTime time.Duration,
) *StateManager {
	// the max number of outputs in a tx is 127, therefore, when creating the splitting tx, we can have at most
	// 126 prepared outputs (+1 remainder output).
	if supplyOutputsCount > MaxFaucetOutputsCount {
		supplyOutputsCount = MaxFaucetOutputsCount
	}
	// number of outputs for each split supply transaction is also limited by the max num of outputs
	if splittingMultiplier > MaxFaucetOutputsCount {
		splittingMultiplier = MaxFaucetOutputsCount
	}

	fState := newFundingState()
	pState := newPreparingState(seed)

	res := &StateManager{
		tokensPerRequest:                tokensPerRequest,
		targetSupplyOutputsCount:        supplyOutputsCount,
		targetFundingOutputsCount:       supplyOutputsCount * splittingMultiplier,
		replenishThreshold:              float64(supplyOutputsCount*splittingMultiplier) * MinFundingOutputsPercentage,
		splittingMultiplier:             splittingMultiplier,
		maxTxBookedAwaitTime:            maxTxBookedTime,
		tokensPerSupplyOutput:           tokensPerRequest * splittingMultiplier,
		tokensUsedOnSupplyReplenishment: tokensPerRequest * splittingMultiplier * supplyOutputsCount,

		fundingState:       fState,
		replenishmentState: pState,
	}

	return res
}

// DeriveStateFromTangle derives the faucet state from a synchronized Tangle.
//  - remainder output should always sit on address 0.
//  - supply outputs should be held on address indices 1-126
//  - funding outputs start from address index 127
//  - if no funding outputs are found, the faucet creates them from the remainder output.
func (s *StateManager) DeriveStateFromTangle(ctx context.Context) (err error) {
	s.replenishmentState.IsReplenishing.Set()
	defer s.replenishmentState.IsReplenishing.UnSet()

	if err = s.findUnspentRemainderOutput(); err != nil {
		return
	}

	endIndex := (Parameters.GenesisTokenAmount-s.replenishmentState.RemainderOutputBalance())/s.tokensPerRequest + MaxFaucetOutputsCount
	Plugin.LogInfof("Set last funding output address index to %d (%d outputs have been prepared in the faucet's lifetime)", endIndex, endIndex-MaxFaucetOutputsCount)

	s.replenishmentState.SetLastFundingOutputAddressIndex(endIndex)

	// check for any unfinished replenishments and use all available supply outputs
	if supplyOutputsFound := s.findSupplyOutputs(); supplyOutputsFound > 0 {
		Plugin.LogInfof("Found %d available supply outputs", s.replenishmentState.SupplyOutputsCount())
		Plugin.LogInfo("Will replenish funding outputs with them...")
		if err = s.replenishFundingOutputs(); err != nil {
			return
		}
	}
	foundFundingOutputs := s.findFundingOutputs()

	if len(foundFundingOutputs) != 0 {
		// save all already prepared outputs into the state manager
		Plugin.LogInfof("Found and restored %d funding outputs", len(foundFundingOutputs))
		s.saveFundingOutputs(foundFundingOutputs)
	}

	if s.replenishThresholdReached() {
		Plugin.LogInfof("Preparing more funding outputs...")
		if err = s.handleReplenishmentErrors(s.replenishSupplyAndFundingOutputs()); err != nil {
			return
		}
	}

	Plugin.LogInfof("Added new funding outputs, last used address index is %d", s.replenishmentState.GetLastFundingOutputAddressIndex())
	Plugin.LogInfof("There are currently %d funding outputs available", s.fundingState.FundingOutputsCount())
	Plugin.LogInfof("Remainder output %s has %d tokens available", s.replenishmentState.RemainderOutputID().Base58(), s.replenishmentState.RemainderOutputBalance())

	return err
}

// FulFillFundingRequest fulfills a faucet request by spending the next funding output to the requested address.
// Mana of the transaction is pledged to the requesting node.
func (s *StateManager) FulFillFundingRequest(faucetReq *faucet.Payload) (*tangleold.Block, string, error) {
	if s.replenishThresholdReached() {
		// wait for replenishment to finish if there is no funding outputs prepared
		waitForPreparation := s.fundingState.FundingOutputsCount() == 0
		s.signalReplenishmentNeeded(waitForPreparation)
	}

	// get an output that we can spend
	fundingOutput, fErr := s.fundingState.GetFundingOutput()
	// we don't have funding outputs
	if errors.Is(fErr, ErrNotEnoughFundingOutputs) {
		err := errors.Errorf("failed to gather funding outputs: %w", fErr)
		return nil, "", err
	}

	tx := s.prepareFaucetTransaction(faucetReq.Address(), fundingOutput, faucetReq.AccessManaPledgeID(), faucetReq.ConsensusManaPledgeID())

	// issue funding request
	m, err := s.issueTx(tx)
	if err != nil {
		return nil, "", err
	}
	txID := tx.ID().Base58()

	return m, txID, nil
}

// replenishThresholdReached checks if the replenishment threshold is reached by examining the available
// funding outputs count against the wanted target funding outputs count.
func (s *StateManager) replenishThresholdReached() bool {
	return uint64(s.fundingState.FundingOutputsCount()) < uint64(s.replenishThreshold)
}

// signalReplenishmentNeeded triggers a replenishment of funding outputs if none is currently running.
// if wait is true it awaits for funds to be prepared to not drop requests and block the queue.
func (s *StateManager) signalReplenishmentNeeded(wait bool) {
	if s.replenishmentState.IsReplenishing.SetToIf(false, true) {
		go func() {
			Plugin.LogInfof("Preparing more funding outputs due to replenishment threshold reached...")
			_ = s.handleReplenishmentErrors(s.replenishSupplyAndFundingOutputs())
		}()
	}
	// waits until preparation of funds will finish
	if wait {
		s.replenishmentState.Wait()
	}
}

// prepareFaucetTransaction prepares a funding faucet transaction that spends fundingOutput to destAddr and pledges
// mana to pledgeID.
func (s *StateManager) prepareFaucetTransaction(destAddr devnetvm.Address, fundingOutput *FaucetOutput, accessManaPledgeID, consensusManaPledgeID identity.ID) (tx *devnetvm.Transaction) {
	inputs := devnetvm.NewInputs(devnetvm.NewUTXOInput(fundingOutput.ID))

	outputs := devnetvm.NewOutputs(devnetvm.NewSigLockedColoredOutput(
		devnetvm.NewColoredBalances(
			map[devnetvm.Color]uint64{
				devnetvm.ColorIOTA: s.tokensPerRequest,
			}),
		destAddr,
	))

	essence := devnetvm.NewTransactionEssence(
		0,
		clock.SyncedTime(),
		accessManaPledgeID,
		consensusManaPledgeID,
		devnetvm.NewInputs(inputs...),
		devnetvm.NewOutputs(outputs...),
	)

	w := wallet{keyPair: *s.replenishmentState.seed.KeyPair(fundingOutput.AddressIndex)}
	unlockBlock := devnetvm.NewSignatureUnlockBlock(w.sign(essence))

	tx = devnetvm.NewTransaction(
		essence,
		devnetvm.UnlockBlocks{unlockBlock},
	)
	return
}

// saveFundingOutputs saves the given slice of indices in StateManager and updates lastFundingOutputAddressIndex.
func (s *StateManager) saveFundingOutputs(fundingOutputs []*FaucetOutput) {
	for _, fOutput := range fundingOutputs {
		s.fundingState.FundingOutputsAdd(fOutput)
	}
}

// findFundingOutputs looks for funding outputs.
func (s *StateManager) findFundingOutputs() []*FaucetOutput {
	foundPreparedOutputs := make([]*FaucetOutput, 0)

	var start, end uint64
	end = s.replenishmentState.GetLastFundingOutputAddressIndex()
	if start = end - s.targetFundingOutputsCount; start <= MaxFaucetOutputsCount {
		start = MaxFaucetOutputsCount + 1
	}

	if start >= end {
		Plugin.LogInfof("No need to search for existing funding outputs, since the faucet is freshly initialized")
		return foundPreparedOutputs
	}

	Plugin.LogInfof("Looking for existing funding outputs in address range %d to %d...", start, end)

	for i := start; i <= end; i++ {
		deps.Indexer.CachedAddressOutputMappings(s.replenishmentState.seed.Address(i).Address()).Consume(func(mapping *indexer.AddressOutputMapping) {
			deps.Tangle.Ledger.Storage.CachedOutput(mapping.OutputID()).Consume(func(output utxo.Output) {
				deps.Tangle.Ledger.Storage.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *ledger.OutputMetadata) {
					if !outputMetadata.IsSpent() {
						outputEssence := output.(devnetvm.Output)

						iotaBalance, colorExist := outputEssence.Balances().Get(devnetvm.ColorIOTA)
						if !colorExist {
							return
						}
						if iotaBalance == s.tokensPerRequest {
							// we found a prepared output
							foundPreparedOutputs = append(foundPreparedOutputs, &FaucetOutput{
								ID:           output.ID(),
								Balance:      iotaBalance,
								Address:      outputEssence.Address(),
								AddressIndex: i,
							})
						}
					}
				})
			})
		})
	}

	Plugin.LogInfof("Found %d funding outputs in the Tangle", len(foundPreparedOutputs))
	Plugin.LogInfof("Looking for funding outputs in the Tangle... DONE")
	return foundPreparedOutputs
}

// findUnspentRemainderOutput finds the remainder output and updates the state manager.
func (s *StateManager) findUnspentRemainderOutput() error {
	var foundRemainderOutput *FaucetOutput

	remainderAddress := s.replenishmentState.seed.Address(RemainderAddressIndex).Address()

	// remainder output should sit on address 0
	deps.Indexer.CachedAddressOutputMappings(remainderAddress).Consume(func(mapping *indexer.AddressOutputMapping) {
		deps.Tangle.Ledger.Storage.CachedOutput(mapping.OutputID()).Consume(func(output utxo.Output) {
			deps.Tangle.Ledger.Storage.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *ledger.OutputMetadata) {
				if !outputMetadata.IsSpent() && deps.Tangle.Ledger.Utils.OutputConfirmationState(outputMetadata.ID()).IsAccepted() {
					outputEssence := output.(devnetvm.Output)

					iotaBalance, ok := outputEssence.Balances().Get(devnetvm.ColorIOTA)
					if !ok || iotaBalance < uint64(minFaucetBalanceMultiplier*float64(Parameters.GenesisTokenAmount)) {
						return
					}
					if foundRemainderOutput != nil && iotaBalance < foundRemainderOutput.Balance {
						// when multiple "big" unspent outputs sit on this address, take the biggest one
						return
					}
					foundRemainderOutput = &FaucetOutput{
						ID:           outputEssence.ID(),
						Balance:      iotaBalance,
						Address:      outputEssence.Address(),
						AddressIndex: RemainderAddressIndex,
					}
				}
			})
		})
	})
	if foundRemainderOutput == nil {
		return errors.Errorf("can't find an output on address %s that has at least %d tokens", remainderAddress.Base58(), int(minFaucetBalanceMultiplier*float64(Parameters.GenesisTokenAmount)))
	}
	s.replenishmentState.SetRemainderOutput(foundRemainderOutput)

	return nil
}

// findSupplyOutputs looks for targetSupplyOutputsCount number of outputs and updates the StateManager.
func (s *StateManager) findSupplyOutputs() uint64 {
	var foundSupplyCount uint64
	var foundOnCurrentAddress bool

	// supply outputs should sit on addresses 1-126
	for supplyAddr := uint64(1); supplyAddr < MaxFaucetOutputsCount+1; supplyAddr++ {
		supplyAddress := s.replenishmentState.seed.Address(supplyAddr).Address()
		// make sure only one output per address will be added
		foundOnCurrentAddress = false

		deps.Indexer.CachedAddressOutputMappings(supplyAddress).Consume(func(mapping *indexer.AddressOutputMapping) {
			deps.Tangle.Ledger.Storage.CachedOutput(mapping.OutputID()).Consume(func(output utxo.Output) {
				if foundOnCurrentAddress {
					return
				}
				if deps.Tangle.Utils.ConfirmedConsumer(output.ID()) == utxo.EmptyTransactionID &&
					deps.Tangle.Ledger.Utils.OutputConfirmationState(output.ID()).IsAccepted() {
					outputEssence := output.(devnetvm.Output)

					iotaBalance, ok := outputEssence.Balances().Get(devnetvm.ColorIOTA)
					if !ok || iotaBalance != s.tokensPerSupplyOutput {
						return
					}
					supplyOutput := &FaucetOutput{
						ID:           outputEssence.ID(),
						Balance:      iotaBalance,
						Address:      outputEssence.Address(),
						AddressIndex: supplyAddr,
					}
					s.replenishmentState.AddSupplyOutput(supplyOutput)
					foundSupplyCount++
					foundOnCurrentAddress = true
				}
			})
		})
	}

	return foundSupplyCount
}

// replenishSupplyAndFundingOutputs create a supply transaction splitting up the remainder output to targetSupplyOutputsCount outputs plus a new remainder output.
// After the supply transaction is confirmed it uses each supply output and splits it for splittingMultiplier many times to generate funding outputs.
// After confirmation of each splitting transaction, outputs are added to fundingOutputs list.
// The faucet remainder is stored on address 0. Next 126 indexes are reserved for supply outputs.
func (s *StateManager) replenishSupplyAndFundingOutputs() (err error) {
	s.replenishmentState.WaitGroup.Add(1)
	defer s.replenishmentState.WaitGroup.Done()

	defer s.replenishmentState.IsReplenishing.UnSet()

	if err = s.findUnspentRemainderOutput(); err != nil {
		return errors.Errorf("%w: %w", ErrMissingRemainderOutput, err)
	}

	if !s.enoughFundsForSupplyReplenishment() {
		err = ErrNotEnoughFunds
		return
	}

	if err = s.replenishSupplyOutputs(); err != nil {
		return errors.Errorf("%w: %w", ErrSupplyPreparationFailed, err)
	}

	if err = s.replenishFundingOutputs(); errors.Is(err, ErrSplittingFundsFailed) {
		return err
	}
	return nil
}

func (s *StateManager) handleReplenishmentErrors(err error) error {
	if err != nil {
		if errors.Is(err, ErrSplittingFundsFailed) {
			err = errors.Errorf("failed to prepare more funding outputs: %w", err)
			Plugin.LogError(err)
			return err
		}
		if errors.Is(err, ErrConfirmationTimeoutExpired) {
			Plugin.LogInfof("Preparing more funding outputs partially successful: %w", err)
		}
	}
	Plugin.LogInfof("Preparing more outputs... DONE")
	return err
}

// enoughFundsForSupplyReplenishment indicates if there are enough funds left to commence a supply replenishment.
func (s *StateManager) enoughFundsForSupplyReplenishment() bool {
	return s.replenishmentState.RemainderOutputBalance() >= s.tokensUsedOnSupplyReplenishment
}

// replenishSupplyOutputs takes the faucet remainder output and splits it up to create supply outputs that will be used for replenishing the funding outputs.
func (s *StateManager) replenishSupplyOutputs() (err error) {
	errChan := make(chan error)
	listenerAttachedChan := make(chan types.Empty)
	s.splittingEnv = newSplittingEnv()

	go s.updateStateOnConfirmation(1, errChan, listenerAttachedChan)
	<-listenerAttachedChan
	if _, ok := preparingWorkerPool.TrySubmit(s.supplyTransactionElements, errChan); !ok {
		Plugin.LogWarn("supply replenishment task not submitted, queue is full")
	}

	// wait for updateStateOnConfirmation to return
	return <-s.splittingEnv.listeningFinished
}

// prepareTransactionTask function for preparation workerPool that uses: provided callback function (param 0)
// to create either supply or split transaction, error channel (param 1) to signal failure during preparation
// or issuance and decrement number of expected confirmations.
func (s *StateManager) prepareTransactionTask(task workerpool.Task) {
	transactionElementsCallback := task.Param(0).(func() (inputs devnetvm.Inputs, outputs devnetvm.Outputs, w wallet, err error))
	preparationFailed := task.Param(1).(chan error)

	tx, err := s.createSplittingTx(transactionElementsCallback)
	if err != nil {
		preparationFailed <- err
		return
	}

	s.splittingEnv.AddIssuedTxID(tx.ID())
	if _, err = s.issueTx(tx); err != nil {
		preparationFailed <- err
		return
	}
}

// replenishFundingOutputs splits available supply outputs to funding outputs.
// It listens for transaction confirmation and in parallel submits transaction preparation and issuance to the worker pool.
func (s *StateManager) replenishFundingOutputs() (err error) {
	errChan := make(chan error)
	listenerAttachedChan := make(chan types.Empty)
	supplyToProcess := uint64(s.replenishmentState.SupplyOutputsCount())
	s.splittingEnv = newSplittingEnv()

	go s.updateStateOnConfirmation(supplyToProcess, errChan, listenerAttachedChan)
	<-listenerAttachedChan

	for i := uint64(0); i < supplyToProcess; i++ {
		if _, ok := preparingWorkerPool.TrySubmit(s.splittingTransactionElements, errChan); !ok {
			Plugin.LogWarn("funding outputs replenishment task not submitted, queue is full")
		}
	}

	// wait for updateStateOnConfirmation to return
	return <-s.splittingEnv.listeningFinished
}

// updateStateOnConfirmation listens for the confirmation and updates the faucet internal state.
// Listening is finished when all issued transactions are confirmed or when the awaiting time is up.
func (s *StateManager) updateStateOnConfirmation(txNumToProcess uint64, preparationFailure <-chan error, listenerAttached chan<- types.Empty) {
	Plugin.LogInfof("Start listening for confirmation")
	// buffered channel will store all confirmed transactions
	txConfirmed := make(chan utxo.TransactionID, txNumToProcess) // length is s.targetSupplyOutputsCount or 1

	monitorTxAcceptance := event.NewClosure(func(event *ledger.TransactionAcceptedEvent) {
		txID := event.TransactionID
		if s.splittingEnv.WasIssuedInThisPreparation(txID) {
			txConfirmed <- txID
		}
	})

	// listen on confirmation
	deps.Tangle.Ledger.Events.TransactionAccepted.Attach(monitorTxAcceptance)
	defer deps.Tangle.Ledger.Events.TransactionAccepted.Detach(monitorTxAcceptance)

	ticker := time.NewTicker(WaitForAcceptance)
	defer ticker.Stop()

	listenerAttached <- types.Empty{}

	// issuedCount indicates number of  transactions issued without any errors, declared with max value,
	// decremented whenever failure is signaled through the preparationFailure channel
	issuedCount := txNumToProcess

	// waiting for transactions to be confirmed
	for {
		select {
		case confirmedTx := <-txConfirmed:
			finished := s.onConfirmation(confirmedTx, issuedCount)
			if finished {
				s.splittingEnv.listeningFinished <- nil
				return
			}
		case <-ticker.C:
			finished, err := s.onTickerCheckMaxAttempts(issuedCount)
			if finished {
				s.splittingEnv.listeningFinished <- err
				return
			}
		case err := <-preparationFailure:
			Plugin.LogErrorf("transaction preparation failed: %s", err)
			issuedCount--
		case <-s.shutdownSignal:
			s.splittingEnv.listeningFinished <- nil
		}
	}
}

func (s *StateManager) onTickerCheckMaxAttempts(issuedCount uint64) (finished bool, err error) {
	if s.splittingEnv.timeoutCount.Load() >= MaxWaitAttempts {
		if s.splittingEnv.confirmedCount.Load() == 0 {
			err = ErrSplittingFundsFailed
			return true, err
		}
		return true, errors.Errorf("confirmed %d and saved %d out of %d issued transactions: %w", s.splittingEnv.confirmedCount.Load(), s.splittingEnv.updateStateCount.Load(), issuedCount, ErrConfirmationTimeoutExpired)
	}
	s.splittingEnv.timeoutCount.Add(1)
	return false, err
}

func (s *StateManager) onConfirmation(confirmedTx utxo.TransactionID, issuedCount uint64) (finished bool) {
	s.splittingEnv.confirmedCount.Add(1)
	err := s.updateState(confirmedTx)
	if err == nil {
		s.splittingEnv.updateStateCount.Add(1)
	}
	// all issued transactions have been confirmed
	if s.splittingEnv.confirmedCount.Load() == issuedCount {
		return true
	}
	return false
}

// updateState takes a confirmed transaction (splitting or supply tx), and updates the faucet internal state based on its content.
func (s *StateManager) updateState(transactionID utxo.TransactionID) (err error) {
	deps.Tangle.Ledger.Storage.CachedTransaction(transactionID).Consume(func(transaction utxo.Transaction) {
		tx, ok := transaction.(*devnetvm.Transaction)
		if !ok {
			return
		}

		newFaucetRemainderBalance := s.replenishmentState.RemainderOutputBalance() - s.tokensUsedOnSupplyReplenishment

		// derive information from outputs
		for _, output := range tx.Essence().Outputs() {
			iotaBalance, hasIota := output.Balances().Get(devnetvm.ColorIOTA)
			if !hasIota {
				err = errors.Errorf("tx outputs don't have IOTA balance ")
				return
			}
			switch iotaBalance {
			case s.tokensPerRequest:
				s.fundingState.FundingOutputsAdd(&FaucetOutput{
					ID:           output.ID(),
					Balance:      iotaBalance,
					Address:      output.Address(),
					AddressIndex: s.replenishmentState.GetAddressToIndex(output.Address().Base58()),
				})
			case newFaucetRemainderBalance:
				s.replenishmentState.SetRemainderOutput(&FaucetOutput{
					ID:           output.ID(),
					Balance:      iotaBalance,
					Address:      output.Address(),
					AddressIndex: s.replenishmentState.GetAddressToIndex(output.Address().Base58()),
				})
			case s.tokensPerSupplyOutput:
				s.replenishmentState.AddSupplyOutput(&FaucetOutput{
					ID:           output.ID(),
					Balance:      iotaBalance,
					Address:      output.Address(),
					AddressIndex: s.replenishmentState.GetAddressToIndex(output.Address().Base58()),
				})
			default:
				err = errors.Errorf("tx %s should not have output with balance %d", transactionID.Base58(), iotaBalance)
				return
			}
		}
	})

	return err
}

// createSplittingTx creates splitting transaction based on provided callback function.
func (s *StateManager) createSplittingTx(transactionElementsCallback func() (devnetvm.Inputs, devnetvm.Outputs, wallet, error)) (*devnetvm.Transaction, error) {
	inputs, outputs, w, err := transactionElementsCallback()
	if err != nil {
		return nil, err
	}
	essence := devnetvm.NewTransactionEssence(
		0,
		clock.SyncedTime(),
		deps.Local.ID(),
		// consensus mana is pledged to EmptyNodeID
		identity.ID{},
		devnetvm.NewInputs(inputs...),
		devnetvm.NewOutputs(outputs...),
	)

	unlockBlock := devnetvm.NewSignatureUnlockBlock(w.sign(essence))

	tx := devnetvm.NewTransaction(
		essence,
		devnetvm.UnlockBlocks{unlockBlock},
	)
	return tx, nil
}

// supplyTransactionElements is a callback function used during supply transaction creation.
// It takes the current remainder output and creates a supply transaction into targetSupplyOutputsCount
// outputs and one remainder output. It uses address indices 1 to targetSupplyOutputsCount because each address in
// a transaction output has to be unique and can prepare at most MaxFaucetOutputsCount supply outputs at once.
func (s *StateManager) supplyTransactionElements() (inputs devnetvm.Inputs, outputs devnetvm.Outputs, w wallet, err error) {
	inputs = devnetvm.NewInputs(devnetvm.NewUTXOInput(s.replenishmentState.RemainderOutputID()))
	// prepare targetSupplyOutputsCount number of supply outputs for further splitting.
	outputs = make(devnetvm.Outputs, 0, s.targetSupplyOutputsCount+1)

	// all funding outputs will land on supply addresses 1 to 126
	for index := uint64(1); index < s.targetSupplyOutputsCount+1; index++ {
		outputs = append(outputs, s.createOutput(s.replenishmentState.seed.Address(index).Address(), s.tokensPerSupplyOutput))
		s.replenishmentState.AddAddressToIndex(s.replenishmentState.seed.Address(index).Address().Base58(), index)
	}

	// add the remainder output
	remainder := s.replenishmentState.RemainderOutputBalance() - s.tokensPerSupplyOutput*s.targetSupplyOutputsCount
	outputs = append(outputs, s.createOutput(s.replenishmentState.seed.Address(RemainderAddressIndex).Address(), remainder))

	w = wallet{keyPair: *s.replenishmentState.seed.KeyPair(RemainderAddressIndex)}
	return
}

// splittingTransactionElements is a callback function used during creation of splitting transactions.
// It splits a supply output into funding outputs and uses lastFundingOutputAddressIndex to derive their target address.
func (s *StateManager) splittingTransactionElements() (inputs devnetvm.Inputs, outputs devnetvm.Outputs, w wallet, err error) {
	supplyOutput, err := s.replenishmentState.NextSupplyOutput()
	if err != nil {
		err = errors.Errorf("could not retrieve supply output: %w", err)
		return
	}

	inputs = devnetvm.NewInputs(devnetvm.NewUTXOInput(supplyOutput.ID))
	outputs = make(devnetvm.Outputs, 0, s.splittingMultiplier)

	for i := uint64(0); i < s.splittingMultiplier; i++ {
		index := s.replenishmentState.IncrLastFundingOutputAddressIndex()
		addr := s.replenishmentState.seed.Address(index).Address()
		outputs = append(outputs, s.createOutput(addr, s.tokensPerRequest))
		s.replenishmentState.AddAddressToIndex(addr.Base58(), index)
	}
	w = wallet{keyPair: *s.replenishmentState.seed.KeyPair(supplyOutput.AddressIndex)}

	return
}

// createOutput creates an output based on provided address and balance.
func (s *StateManager) createOutput(addr devnetvm.Address, balance uint64) devnetvm.Output {
	return devnetvm.NewSigLockedColoredOutput(
		devnetvm.NewColoredBalances(
			map[devnetvm.Color]uint64{
				devnetvm.ColorIOTA: balance,
			}),
		addr,
	)
}

// issueTx issues a transaction to the Tangle and waits for it to become booked.
func (s *StateManager) issueTx(tx *devnetvm.Transaction) (blk *tangleold.Block, err error) {
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
	blk, err = blocklayer.AwaitBlockToBeBooked(issueTransaction, tx.ID(), s.maxTxBookedAwaitTime)
	if err != nil {
		return nil, errors.Errorf("%w: tx %s", err, tx.ID().String())
	}
	return blk, nil
}

// splittingEnv provides variables used for synchronization during splitting transactions.
type splittingEnv struct {
	// preparedTxID is a map that stores prepared and issued transaction IDs
	issuedTxIDs map[utxo.TransactionID]types.Empty
	sync.RWMutex

	// channel to signal that listening has finished
	listeningFinished chan error

	// counts confirmed transactions during listening
	confirmedCount *atomic.Uint64

	// counts successful splits
	updateStateCount *atomic.Uint64

	// counts max attempts while listening for confirmation
	timeoutCount *atomic.Uint64
}

func newSplittingEnv() *splittingEnv {
	return &splittingEnv{
		issuedTxIDs:       make(map[utxo.TransactionID]types.Empty),
		listeningFinished: make(chan error),
		confirmedCount:    atomic.NewUint64(0),
		updateStateCount:  atomic.NewUint64(0),
		timeoutCount:      atomic.NewUint64(0),
	}
}

// WasIssuedInThisPreparation indicates if given transaction was issued during this lifespan of splittingEnv.
func (s *splittingEnv) WasIssuedInThisPreparation(transactionID utxo.TransactionID) bool {
	s.RLock()
	defer s.RUnlock()

	_, ok := s.issuedTxIDs[transactionID]
	return ok
}

// IssuedTransactionsCount returns how many transactions was issued this far.
func (s *splittingEnv) IssuedTransactionsCount() uint64 {
	s.RLock()
	defer s.RUnlock()

	return uint64(len(s.issuedTxIDs))
}

// AddIssuedTxID adds transactionID to the issuedTxIDs map.
func (s *splittingEnv) AddIssuedTxID(txID utxo.TransactionID) {
	s.Lock()
	defer s.Unlock()
	s.issuedTxIDs[txID] = types.Void
}

// fundingState manages fundingOutputs and its mutex.
type fundingState struct {
	// ordered list of available outputs to fund faucet requests
	fundingOutputs *list.List

	sync.RWMutex
}

func newFundingState() *fundingState {
	state := &fundingState{
		fundingOutputs: list.New(),
	}

	return state
}

// FundingOutputsCount returns the number of available outputs that can be used to fund a request.
func (f *fundingState) FundingOutputsCount() int {
	f.RLock()
	defer f.RUnlock()

	return f.fundingOutputsCount()
}

// FundingOutputsAdd adds FaucetOutput to the fundingOutputs list.
func (f *fundingState) FundingOutputsAdd(fundingOutput *FaucetOutput) {
	f.Lock()
	defer f.Unlock()

	f.fundingOutputs.PushBack(fundingOutput)
}

// GetFundingOutput returns the first funding output in the list.
func (f *fundingState) GetFundingOutput() (fundingOutput *FaucetOutput, err error) {
	f.Lock()
	defer f.Unlock()

	if f.fundingOutputsCount() < 1 {
		return nil, ErrNotEnoughFundingOutputs
	}
	fundingOutput = f.fundingOutputs.Remove(f.fundingOutputs.Front()).(*FaucetOutput)
	return
}

func (f *fundingState) fundingOutputsCount() int {
	return f.fundingOutputs.Len()
}

// replenishmentState keeps all variables and related methods used to track faucet state during replenishment.
type replenishmentState struct {
	// output that holds the remainder funds to the faucet, should always be on address 0
	remainderOutput *FaucetOutput
	// outputs that hold funds during the replenishment phase, filled in only with outputs needed for next split, should always be on address 1
	supplyOutputs *list.List
	// the last funding output address index, should start from MaxFaucetOutputsCount + 1
	// when we prepare new funding outputs, we start from lastFundingOutputAddressIndex + 1
	lastFundingOutputAddressIndex uint64
	// mapping base58 encoded addresses to their indices
	addressToIndex map[string]uint64
	// the seed instance of the faucet holding the tokens
	seed *walletseed.Seed
	// IsReplenishing indicates if faucet is currently replenishing the next batch of funding outputs
	IsReplenishing typeutils.AtomicBool

	// is used when fulfilling request for waiting for more funds in case they were not prepared on time
	sync.WaitGroup
	// ensures that fields related to new funds creation can be accesses by only one goroutine at the same time
	sync.RWMutex
}

func newPreparingState(seed *walletseed.Seed) *replenishmentState {
	state := &replenishmentState{
		seed: seed,
		addressToIndex: map[string]uint64{
			seed.Address(RemainderAddressIndex).Address().Base58(): RemainderAddressIndex,
		},
		lastFundingOutputAddressIndex: MaxFaucetOutputsCount,
		supplyOutputs:                 list.New(),
		remainderOutput:               nil,
	}
	return state
}

// RemainderOutputBalance returns the balance value of remainderOutput.
func (p *replenishmentState) RemainderOutputBalance() uint64 {
	p.RLock()
	defer p.RUnlock()
	return p.remainderOutput.Balance
}

// RemainderOutputID returns the OutputID of remainderOutput.
func (p *replenishmentState) RemainderOutputID() utxo.OutputID {
	p.RLock()
	defer p.RUnlock()
	id := p.remainderOutput.ID
	return id
}

// SetRemainderOutput sets provided output as remainderOutput.
func (p *replenishmentState) SetRemainderOutput(output *FaucetOutput) {
	p.Lock()
	defer p.Unlock()

	p.remainderOutput = output
}

// NextSupplyOutput returns the first supply address in the list.
func (p *replenishmentState) NextSupplyOutput() (supplyOutput *FaucetOutput, err error) {
	p.Lock()
	defer p.Unlock()

	if p.supplyOutputsCount() < 1 {
		return nil, ErrNotEnoughSupplyOutputs
	}
	element := p.supplyOutputs.Front()
	supplyOutput = p.supplyOutputs.Remove(element).(*FaucetOutput)
	return
}

// SupplyOutputsCount returns the number of available outputs that can be split to prepare more faucet funds.
func (p *replenishmentState) SupplyOutputsCount() int {
	p.RLock()
	defer p.RUnlock()

	return p.supplyOutputsCount()
}

func (p *replenishmentState) supplyOutputsCount() int {
	return p.supplyOutputs.Len()
}

// AddSupplyOutput adds FaucetOutput to the supplyOutputs.
func (p *replenishmentState) AddSupplyOutput(output *FaucetOutput) {
	p.Lock()
	defer p.Unlock()

	p.supplyOutputs.PushBack(output)
}

// IncrLastFundingOutputAddressIndex increments and returns the new lastFundingOutputAddressIndex value.
func (p *replenishmentState) IncrLastFundingOutputAddressIndex() uint64 {
	p.Lock()
	defer p.Unlock()

	p.lastFundingOutputAddressIndex++
	return p.lastFundingOutputAddressIndex
}

// GetLastFundingOutputAddressIndex returns current lastFundingOutputAddressIndex value.
func (p *replenishmentState) GetLastFundingOutputAddressIndex() uint64 {
	p.RLock()
	defer p.RUnlock()

	return p.lastFundingOutputAddressIndex
}

// SetLastFundingOutputAddressIndex sets new lastFundingOutputAddressIndex.
func (p *replenishmentState) SetLastFundingOutputAddressIndex(index uint64) {
	p.Lock()
	defer p.Unlock()

	p.lastFundingOutputAddressIndex = index
}

// GetAddressToIndex returns index for provided address based on addressToIndex map.
func (p *replenishmentState) GetAddressToIndex(addr string) uint64 {
	p.RLock()
	defer p.RUnlock()

	return p.addressToIndex[addr]
}

// AddAddressToIndex adds address and corresponding index to the addressToIndex map.
func (p *replenishmentState) AddAddressToIndex(addr string, index uint64) {
	p.Lock()
	defer p.Unlock()

	p.addressToIndex[addr] = index
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
