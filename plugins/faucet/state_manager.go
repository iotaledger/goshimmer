package faucet

import (
	"container/list"
	"github.com/iotaledger/hive.go/types"
	"github.com/iotaledger/hive.go/workerpool"
	"go.uber.org/atomic"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/typeutils"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"

	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/faucet"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

const (
	// GenesisTokenAmount is the total supply.
	GenesisTokenAmount = 1000000000000000

	// RemainderAddressIndex is the RemainderAddressIndex.
	RemainderAddressIndex = 0

	// MinimumFaucetBalance defines the minimum token amount required, before the faucet stops operating.
	MinimumFaucetBalance = 0.1 * GenesisTokenAmount

	// MinimumFaucetRemindersPercentageLeft defines the minimum percentage of prepared fundingReminders that triggers funds preparation
	MinimumFaucetRemindersPercentageLeft = 30

	// MaxFaucetOutputsCount defines the max outputs count for the Faucet as the ledgerstate.MaxOutputCount -1 remainder output.
	MaxFaucetOutputsCount = ledgerstate.MaxOutputCount - 1

	// WaitForConfirmation defines the wait time before considering a transaction confirmed.
	WaitForConfirmation = 10 * time.Second

	// totalPercentage constant used to calculate percentage of funds left in the faucet
	totalPercentage = 100
)

// region FaucetOutput

// FaucetOutput represents an output controlled by the faucet.
type FaucetOutput struct {
	ID           ledgerstate.OutputID
	Balance      uint64
	Address      ledgerstate.Address
	AddressIndex uint64
}

// endregion

// region StateManager

// StateManager manages the funds and outputs of the faucet. Can derive its state from a synchronized Tangle, can
// carry out funding requests, and prepares more funding outputs when needed.
type StateManager struct {
	// the amount of tokens to send to every request
	tokensPerRequest uint64
	// number of funding outputs to prepare for supply address that will be break down further if fundingOutputs is short on funds
	preparedOutputsCount uint64
	// number of funding outputs for each output in supply transaction during the splitting period
	splittingMultiplayer uint64

	// the time to await for the transaction fulfilling a funding request
	// to become booked in the value layer
	maxTxBookedAwaitTime time.Duration

	// fulfillState holds fundingOutputs and its mutex
	fulfillState *fulfillState

	// preparingState keeps all variables used to track faucet state during funds preparation
	preparingState *preparingState

	// splittingEnv keeps all variables necessary to split transaction
	splittingEnv *splittingEnv
}

// NewStateManager creates a new state manager for the faucet.
func NewStateManager(
	tokensPerRequest uint64,
	seed *walletseed.Seed,
	preparedOutputsCount uint64,
	splittingMultiplayer uint64,
	maxTxBookedTime time.Duration,
) *StateManager {
	// currently the max number of outputs in a tx is 127, therefore, when creating the splitting tx, we can have at most
	// 126 prepared outputs (+1 remainder output).
	if preparedOutputsCount > MaxFaucetOutputsCount {
		preparedOutputsCount = MaxFaucetOutputsCount
	}
	// number of outputs for each split supply transaction is also limited by the max num of outputs
	if splittingMultiplayer > MaxFaucetOutputsCount {
		splittingMultiplayer = MaxFaucetOutputsCount
	}

	fState := newFulfillState()
	pState := newPreparingState(seed)

	res := &StateManager{
		tokensPerRequest:     tokensPerRequest,
		preparedOutputsCount: preparedOutputsCount,
		splittingMultiplayer: splittingMultiplayer,
		maxTxBookedAwaitTime: maxTxBookedTime,

		fulfillState:   fState,
		preparingState: pState,
	}

	return res
}

// DeriveStateFromTangle derives the faucet state from a synchronized Tangle.
//  - remainder output should always sit on address 0.
//  - supply outputs should be held on addresses 1-126
//  - faucet indexes stats from 127
//   - if no funding outputs are found, the faucet creates them from the remainder output.
func (s *StateManager) DeriveStateFromTangle() (err error) {
	s.preparingState.IsPreparingFunds.Set()
	defer s.preparingState.IsPreparingFunds.UnSet()

	err = s.findUnspentRemainderOutput()
	if err != nil {
		return
	}

	// check for any unfinished funds preparation and use all remaining supply outputs
	err = s.findSupplyOutputs()
	if err == nil {
		err = s.prepareSupplyFunding()
		if err != nil {
			Plugin().LogInfof("Found and complete unfinished funds preparation")
		}
	}

	endIndex := (GenesisTokenAmount-s.preparingState.RemainderOutputBalance())/s.tokensPerRequest + 1
	Plugin().LogInfof("%d indices have already been used based on found remainder output", endIndex)

	foundPreparedOutputs := s.findFundingOutputs(endIndex)

	if len(foundPreparedOutputs) != 0 {
		// save all already prepared outputs into the state manager
		s.saveFundingOutputs(foundPreparedOutputs)
	}

	if s.notEnoughFundsInTheFaucet() {
		Plugin().LogInfof("Preparing more outputs...")
		err = s.prepareMoreFundingOutputs()
		err = s.handlePrepareErrors(err)
	}

	Plugin().LogInfof("Added new funding outputs, last used address index is %d", s.preparingState.GetLastFundingOutputAddressIndex())
	Plugin().LogInfof("There are currently %d prepared outputs in the faucet", s.fulfillState.FundingOutputsCount())
	Plugin().LogInfof("Remainder output %s had %d funds", s.preparingState.RemainderOutputID().Base58(), s.preparingState.RemainderOutputBalance())

	return err
}

// FulFillFundingRequest fulfills a faucet request by spending the next funding output to the requested address.
// Mana of the transaction is pledged to the requesting node.
func (s *StateManager) FulFillFundingRequest(requestMsg *tangle.Message) (m *tangle.Message, txID string, err error) {
	faucetReq := requestMsg.Payload().(*faucet.Request)

	if s.notEnoughFundsInTheFaucet() {
		// wait if there is no outputs prepared
		waitForPreparation := s.fulfillState.FundingOutputsCount() == 0
		s.signalMoreFundingNeeded(waitForPreparation)
	}

	// get an output that we can spend
	fundingOutput, fErr := s.fulfillState.GetFundingOutput()
	// we don't have funding outputs
	if errors.Is(fErr, ErrNotEnoughFundingOutputs) {
		err = errors.Errorf("failed to gather funding outputs: %w", fErr)
		return
	}

	// prepare funding tx, pledge mana to requester
	emptyID := identity.ID{}
	accessManaPledgeID := identity.NewID(requestMsg.IssuerPublicKey())
	consensusManaPledgeID := identity.NewID(requestMsg.IssuerPublicKey())
	if faucetReq.AccessManaPledgeID() != emptyID {
		accessManaPledgeID = faucetReq.AccessManaPledgeID()
	}
	if faucetReq.ConsensusManaPledgeID() != emptyID {
		consensusManaPledgeID = faucetReq.ConsensusManaPledgeID()
	}

	tx := s.prepareFaucetTransaction(faucetReq.Address(), fundingOutput, accessManaPledgeID, consensusManaPledgeID)

	// issue funding request
	m, err = s.issueTX(tx)
	if err != nil {
		return
	}
	txID = tx.ID().Base58()

	return
}

// notEnoughFundsInTheFaucet checks if number of funding outputs is lower than MinimumFaucetRemindersPercentageLeft of total funds prepared at once
func (s *StateManager) notEnoughFundsInTheFaucet() bool {
	return uint64(s.fulfillState.FundingOutputsCount()) < uint64(float64(s.splittingMultiplayer*s.preparedOutputsCount)*float64(MinimumFaucetRemindersPercentageLeft)/totalPercentage)
}

// signalMoreFundingNeeded triggers preparation of faucet funding only if none preparation is currently running
// if wait is true it awaits for funds to be prepared
func (s *StateManager) signalMoreFundingNeeded(wait bool) {
	if s.preparingState.IsPreparingFunds.SetToIf(false, true) {
		go func() {
			Plugin().LogInfof("Preparing more outputs...")
			err := s.prepareMoreFundingOutputs()
			_ = s.handlePrepareErrors(err)
		}()
	}
	// waits until preparation of funds will finish
	if wait {
		s.preparingState.Wait()
	}
}

// prepareFaucetTransaction prepares a funding faucet transaction that spends fundingOutput to destAddr and pledges
// mana to pledgeID.
func (s *StateManager) prepareFaucetTransaction(destAddr ledgerstate.Address, fundingOutput *FaucetOutput, accessManaPledgeID, consensusManaPledgeID identity.ID) (tx *ledgerstate.Transaction) {
	inputs := ledgerstate.NewInputs(ledgerstate.NewUTXOInput(fundingOutput.ID))

	outputs := ledgerstate.NewOutputs(ledgerstate.NewSigLockedColoredOutput(
		ledgerstate.NewColoredBalances(
			map[ledgerstate.Color]uint64{
				ledgerstate.ColorIOTA: s.tokensPerRequest,
			}),
		destAddr,
	),
	)

	essence := ledgerstate.NewTransactionEssence(
		0,
		clock.SyncedTime(),
		accessManaPledgeID,
		consensusManaPledgeID,
		ledgerstate.NewInputs(inputs...),
		ledgerstate.NewOutputs(outputs...),
	)

	w := wallet{keyPair: *s.preparingState.seed.KeyPair(fundingOutput.AddressIndex)}
	unlockBlock := ledgerstate.NewSignatureUnlockBlock(w.sign(essence))

	tx = ledgerstate.NewTransaction(
		essence,
		ledgerstate.UnlockBlocks{unlockBlock},
	)
	return
}

// saveFundingOutputs saves the given slice of indices in StateManager and updates lastFundingOutputAddressIndex.
func (s *StateManager) saveFundingOutputs(fundingOutputs []*FaucetOutput) {
	for _, fOutput := range fundingOutputs {
		s.fulfillState.FundingOutputsAdd(fOutput)
		s.preparingState.UpdateLastFundingOutputAddressIndex(fOutput.AddressIndex)
	}
}

// findFundingOutputs looks for prepared outputs in the tangle.
func (s *StateManager) findFundingOutputs(endIndex uint64) []*FaucetOutput {
	foundPreparedOutputs := make([]*FaucetOutput, 0)

	Plugin().LogInfof("Looking for prepared outputs in the Tangle...")

	for i := MaxFaucetOutputsCount + 1; uint64(i) <= endIndex; i++ {
		messagelayer.Tangle().LedgerState.CachedOutputsOnAddress(s.preparingState.seed.Address(uint64(i)).Address()).Consume(func(output ledgerstate.Output) {
			messagelayer.Tangle().LedgerState.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
				if outputMetadata.ConsumerCount() < 1 {
					iotaBalance, colorExist := output.Balances().Get(ledgerstate.ColorIOTA)
					if !colorExist {
						return
					}
					if iotaBalance == s.tokensPerRequest {
						// we found a prepared output
						foundPreparedOutputs = append(foundPreparedOutputs, &FaucetOutput{
							ID:           output.ID(),
							Balance:      iotaBalance,
							Address:      output.Address(),
							AddressIndex: uint64(i),
						})
					}
				}
			})
		})
	}
	Plugin().LogInfof("Found %d prepared outputs in the Tangle", len(foundPreparedOutputs))
	Plugin().LogInfof("Looking for prepared outputs in the Tangle... DONE")
	return foundPreparedOutputs
}

// findUnspentRemainderOutput finds the remainder output and updates the state manager
func (s *StateManager) findUnspentRemainderOutput() error {
	var foundRemainderOutput *FaucetOutput

	remainderAddress := s.preparingState.seed.Address(RemainderAddressIndex).Address()

	// remainder output should sit on address 0
	messagelayer.Tangle().LedgerState.CachedOutputsOnAddress(remainderAddress).Consume(func(output ledgerstate.Output) {
		messagelayer.Tangle().LedgerState.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
			if outputMetadata.ConfirmedConsumer().Base58() == ledgerstate.GenesisTransactionID.Base58() &&
				outputMetadata.Finalized() {
				iotaBalance, ok := output.Balances().Get(ledgerstate.ColorIOTA)
				if !ok || iotaBalance < MinimumFaucetBalance {
					return
				}
				if foundRemainderOutput != nil && iotaBalance < foundRemainderOutput.Balance {
					// when multiple "big" unspent outputs sit on this address, take the biggest one
					return
				}
				foundRemainderOutput = &FaucetOutput{
					ID:           output.ID(),
					Balance:      iotaBalance,
					Address:      output.Address(),
					AddressIndex: RemainderAddressIndex,
				}
			}
		})
	})
	if foundRemainderOutput == nil {
		return errors.Errorf("can't find an output on address %s that has at least %d tokens", remainderAddress.Base58(), int(MinimumFaucetBalance))
	}
	s.preparingState.SetRemainderOutput(foundRemainderOutput)

	return nil
}

// findSupplyOutputs looks for preparedOutputsCount number of reminders of supply transaction and updates the StateManager
func (s *StateManager) findSupplyOutputs() (err error) {
	var foundSupplyCount uint64
	var foundOnCurrentAddress bool

	// supply outputs should sit on addresses 1-126
	for supplyAddr := uint64(1); supplyAddr < MaxFaucetOutputsCount+1; supplyAddr++ {
		supplyAddress := s.preparingState.seed.Address(supplyAddr).Address()
		// make sure only one output per address will be added
		foundOnCurrentAddress = false

		messagelayer.Tangle().LedgerState.CachedOutputsOnAddress(supplyAddress).Consume(func(output ledgerstate.Output) {
			if foundSupplyCount >= s.splittingMultiplayer || foundOnCurrentAddress {
				// return when enough outputs has been collected or output has been already found on this address
				return
			}
			messagelayer.Tangle().LedgerState.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
				if outputMetadata.ConfirmedConsumer().Base58() == ledgerstate.GenesisTransactionID.Base58() &&
					outputMetadata.Finalized() {
					iotaBalance, ok := output.Balances().Get(ledgerstate.ColorIOTA)
					if !ok || iotaBalance != s.tokensPerRequest*s.splittingMultiplayer {
						return
					}
					supplyOutput := &FaucetOutput{
						ID:           output.ID(),
						Balance:      iotaBalance,
						Address:      output.Address(),
						AddressIndex: supplyAddr,
					}
					s.preparingState.AddSupplyOutput(supplyOutput)
					foundSupplyCount++
					foundOnCurrentAddress = true
				}
			})
		})
	}

	if foundSupplyCount == 0 {
		return errors.Errorf("can't find any supply output that has %d tokens", int(s.tokensPerRequest))
	}
	return nil
}

// prepareMoreFundingOutputs prepares more funding outputs by splitting up the remainder output on preparedOutputsCount outputs plus new reminder.
// and submits supply transactions. After transaction is confirmed it uses each supply output and splits it again for splittingMultiplayer many times.
// After confirmation, outputs are added to fundingOutputs list.
// Reminder is stored on address 0. Next 126 indexes are reserved for supply transaction outputs.
func (s *StateManager) prepareMoreFundingOutputs() (err error) {
	//t0 := time.Now()
	s.preparingState.WaitGroup.Add(1)
	defer s.preparingState.WaitGroup.Done()

	defer s.preparingState.IsPreparingFunds.UnSet()

	// no remainder output present
	err = s.findUnspentRemainderOutput()
	if err != nil {
		return errors.Errorf("%w: %w", ErrMissingRemainderOutput, err)
	}
	// if no error was returned, remainderOutput is not nil anymore

	if s.notEnoughFunds() {
		err = ErrNotEnoughFunds
		return
	}

	err = s.prepareSupplyFunding()
	if err != nil {
		return errors.Errorf("%w: %w", ErrSupplyPreparationFailed, err)
	}

	err = s.splitSupplyTransaction()
	if errors.Is(err, ErrSplittingFundsFailed) {
		return err
	}
	return nil
}

func (s *StateManager) handlePrepareErrors(err error) error {
	if err != nil {
		if errors.Is(err, ErrSplittingFundsFailed) {
			err = errors.Errorf("failed to prepare more outputs: %w", err)
			Plugin().LogError(err)
			return err
		}
		if errors.Is(err, ErrConfirmationTimeoutExpired) {
			Plugin().LogInfof("Preparing more outputs partially successful: %w", err)
		}
		Plugin().LogInfof("Preparing more outputs... DONE")
	}
	return err
}

// isEnoughFunds indicates if there is enough funds to carry on the faucet funds preparation
func (s *StateManager) notEnoughFunds() bool {
	// not enough funds to carry out the operation
	return s.preparingState.RemainderOutputBalance() < s.tokensPerRequest*s.preparedOutputsCount*s.splittingMultiplayer
}

// prepareSupplyFunding takes a remainder output and splits it up to create supply transaction that will be used for further splitting
func (s *StateManager) prepareSupplyFunding() (err error) {
	preparationFailure := make(chan types.Empty)
	s.splittingEnv = newSplittingEnv()
	Plugin().LogInfof("Created new preparation environment for supply")

	go s.splittingEnv.listenOnConfirmationAndUpdateState(s, 1, preparationFailure)
	_, ok := preparingWorkerPool.TrySubmit(s.supplyTransactionElements, preparationFailure)
	if !ok {
		Plugin().LogInfo("supply funding task not submitted, queue is full")
	}
	// wait for listenOnConfirmationAndUpdateState to return
	Plugin().LogInfof("wait for listenOnConfirmationAndUpdateState to return")
	err = <-s.splittingEnv.listeningFinished

	return
}

// prepareTransactionTask function for preparation workerPool that uses provided callback function
// to create either supply or split transaction
func (s *StateManager) prepareTransactionTask(task workerpool.Task) {
	transactionElementsCallback := task.Param(0).(func() (inputs ledgerstate.Inputs, outputs ledgerstate.Outputs, w wallet, err error))
	preparationFailed := task.Param(1).(chan types.Empty)

	tx, err := s.createSplittingTx(transactionElementsCallback)
	if err != nil {
		Plugin().LogInfof("prepareTransactionTask Preparation failed tx %s ", err)
		preparationFailed <- types.Void
		return
	}
	s.splittingEnv.AddIssuedTxID(tx.ID())
	_, err = s.issueTX(tx)
	if err != nil {
		return
	}
	return
}

// splitSupplyTransaction splits further outputs from supply transaction to create fundingReminders.
// It listens for transaction confirmation and submits transaction preparation and issuance to the worker pool.
func (s *StateManager) splitSupplyTransaction() (err error) {
	preparationFailure := make(chan types.Empty)

	supplyToProcess := uint64(s.preparingState.SupplyOutputsCount())
	s.splittingEnv = newSplittingEnv()
	go s.splittingEnv.listenOnConfirmationAndUpdateState(s, supplyToProcess, preparationFailure)

	for i := uint64(0); i < supplyToProcess; i++ {
		preparingWorkerPool.TrySubmit(s.splittingTransactionElements, preparationFailure)
	}

	// wait for listenOnConfirmationAndUpdateState to return
	err = <-s.splittingEnv.listeningFinished
	return
}

// listenOnConfirmationAndUpdateState submits splitting transactions to the worker pool,
// listen for the confirmation and updates the faucet internal state
func (s *splittingEnv) listenOnConfirmationAndUpdateState(manager *StateManager, txNumToProcess uint64, preparationFailure <-chan types.Empty) {
	Plugin().LogInfof("Start listening for confirmation")
	// buffered channel will store all confirmed transactions
	txConfirmed := make(chan ledgerstate.TransactionID, txNumToProcess) // length is s.preparedOutputsCount or 1

	monitorTxConfirmation := events.NewClosure(func(transactionID ledgerstate.TransactionID) {
		if s.WasIssuedInThisPreparation(transactionID) {
			txConfirmed <- transactionID
		}
	})

	// listen on confirmation
	messagelayer.Tangle().LedgerState.UTXODAG.Events().TransactionConfirmed.Attach(monitorTxConfirmation)
	defer messagelayer.Tangle().LedgerState.UTXODAG.Events().TransactionConfirmed.Detach(monitorTxConfirmation)

	// waiting for transactions to be confirmed
	ticker := time.NewTicker(WaitForConfirmation)
	defer ticker.Stop()
	timeoutCounter := 0
	maxWaitAttempts := 50 // 500 s max timeout (if fpc voting is in place)

	// issuedCount if all transactions issued without any errors, declared with max value, updated after all tx issuance
	issuedCount := txNumToProcess

	for {
		select {
		case confirmedTx := <-txConfirmed:
			finished := s.onConfirmation(manager, confirmedTx, issuedCount)
			if finished {
				s.listeningFinished <- nil
				return
			}
		case <-ticker.C:
			err, finished := s.onTickerCheckMaxAttempts(timeoutCounter, maxWaitAttempts, issuedCount)
			if finished {
				s.listeningFinished <- err
				return
			}
		case <-preparationFailure:
			// issued count after all transaction were issued
			issuedCount--
		}
	}
}

func (s *splittingEnv) onTickerCheckMaxAttempts(timeoutCounter int, maxWaitAttempts int, issuedCount uint64) (err error, finished bool) {
	if timeoutCounter >= maxWaitAttempts {
		if s.confirmedCount.Load() == 0 {
			err = ErrSplittingFundsFailed
			return err, true
		}
		return errors.Errorf("confirmed %d and saved %d out of %d issued transactions: %w", s.confirmedCount.Load(), s.updateStateCount.Load(), issuedCount, ErrConfirmationTimeoutExpired), true
	}
	timeoutCounter++
	return nil, false
}

func (s *splittingEnv) onConfirmation(manager *StateManager, confirmedTx ledgerstate.TransactionID, issuedCount uint64) (finished bool) {
	s.confirmedCount.Add(1)
	err := manager.updateState(confirmedTx)
	if err == nil {
		s.updateStateCount.Add(1)
	}
	// all issued transactions has been confirmed
	if s.confirmedCount.Load() == issuedCount {
		return true
	}
	return false
}

// updateState takes a confirmed transaction (splitting tx), and updates the faucet internal state based on its content.
func (s *StateManager) updateState(transactionID ledgerstate.TransactionID) (err error) {
	messagelayer.Tangle().LedgerState.Transaction(transactionID).Consume(func(transaction *ledgerstate.Transaction) {
		remainingBalance := s.preparingState.RemainderOutputBalance() - s.tokensPerRequest*s.preparedOutputsCount*s.splittingMultiplayer
		supplyBalance := s.tokensPerRequest * s.splittingMultiplayer

		// derive information from outputs
		for _, output := range transaction.Essence().Outputs() {
			iotaBalance, hasIota := output.Balances().Get(ledgerstate.ColorIOTA)
			if !hasIota {
				err = errors.Errorf("tx outputs don't have IOTA balance ")
				return
			}
			switch iotaBalance {
			case s.tokensPerRequest:
				s.fulfillState.FundingOutputsAdd(&FaucetOutput{
					ID:           output.ID(),
					Balance:      iotaBalance,
					Address:      output.Address(),
					AddressIndex: s.preparingState.GetAddressToIndex(output.Address().Base58()),
				})
			case remainingBalance:
				s.preparingState.SetRemainderOutput(&FaucetOutput{
					ID:           output.ID(),
					Balance:      iotaBalance,
					Address:      output.Address(),
					AddressIndex: s.preparingState.GetAddressToIndex(output.Address().Base58()),
				})
			case supplyBalance:
				s.preparingState.AddSupplyOutput(&FaucetOutput{
					ID:           output.ID(),
					Balance:      iotaBalance,
					Address:      output.Address(),
					AddressIndex: s.preparingState.GetAddressToIndex(output.Address().Base58()),
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
func (s *StateManager) createSplittingTx(transactionElementsCallback func() (ledgerstate.Inputs, ledgerstate.Outputs, wallet, error)) (*ledgerstate.Transaction, error) {
	// prepare inputs and outputs for supply transaction
	inputs, outputs, w, err := transactionElementsCallback()
	if err != nil {
		return nil, err
	}
	essence := ledgerstate.NewTransactionEssence(
		0,
		clock.SyncedTime(),
		local.GetInstance().ID(),
		// consensus mana is pledged to EmptyNodeID
		identity.ID{},
		ledgerstate.NewInputs(inputs...),
		ledgerstate.NewOutputs(outputs...),
	)

	unlockBlock := ledgerstate.NewSignatureUnlockBlock(w.sign(essence))

	tx := ledgerstate.NewTransaction(
		essence,
		ledgerstate.UnlockBlocks{unlockBlock},
	)
	return tx, nil
}

// supplyTransactionElements is a callback function used during supply transaction creation.
// It takes the current remainder output and creates a first splitting transaction into preparedOutputsCount
// funding outputs and one remainder output. It uses address indices 1 - preparedOutputsCount because each address in
// transaction output has to be unique and can prepare at most MaxFaucetOutputsCount supply outputs at once.
func (s *StateManager) supplyTransactionElements() (inputs ledgerstate.Inputs, outputs ledgerstate.Outputs, w wallet, err error) {
	inputs = ledgerstate.NewInputs(ledgerstate.NewUTXOInput(s.preparingState.RemainderOutputID()))
	// prepare s.preparedOutputsCount number of supply outputs for further splitting.
	outputs = make(ledgerstate.Outputs, 0, s.preparedOutputsCount+1)
	balance := s.tokensPerRequest * s.splittingMultiplayer
	// all funding outputs will land on supply addresses 1 to 126
	for index := uint64(1); index < s.preparedOutputsCount+1; index++ {
		outputs = append(outputs, s.createOutput(s.preparingState.seed.Address(index).Address(), balance))
		s.preparingState.AddAddressToIndex(s.preparingState.seed.Address(index).Address().Base58(), index)
	}
	// add the remainder output
	balance = s.preparingState.RemainderOutputBalance() - s.tokensPerRequest*s.splittingMultiplayer*s.preparedOutputsCount
	outputs = append(outputs, s.createOutput(s.preparingState.seed.Address(RemainderAddressIndex).Address(), balance))
	// signature
	w = wallet{keyPair: *s.preparingState.seed.KeyPair(RemainderAddressIndex)}
	return
}

// splittingTransactionElements is a callback function used during creation of splitting transactions.
// It splits each supply output into funding remainders and uses lastFundingOutputAddressIndex to derive their address.
func (s *StateManager) splittingTransactionElements() (inputs ledgerstate.Inputs, outputs ledgerstate.Outputs, w wallet, err error) {
	reminder, err := s.preparingState.NextSupplyOutput()
	if err != nil {
		err = errors.Errorf("could not retrieve supply output: %w", err)
		return
	}
	inputs = ledgerstate.NewInputs(ledgerstate.NewUTXOInput(reminder.ID))
	// prepare s.splittingMultiplayer number of funding outputs.
	outputs = make(ledgerstate.Outputs, 0, s.splittingMultiplayer)
	// start from the last used funding output address index
	for i := uint64(0); i < s.splittingMultiplayer; i++ {
		index := s.preparingState.IncrLastFundingOutputAddressIndex()
		addr := s.preparingState.seed.Address(index).Address()
		outputs = append(outputs, s.createOutput(addr, s.tokensPerRequest))
		s.preparingState.AddAddressToIndex(addr.Base58(), index)
	}
	// signature
	w = wallet{keyPair: *s.preparingState.seed.KeyPair(reminder.AddressIndex)}

	return
}

func (s *StateManager) createOutput(addr ledgerstate.Address, balance uint64) ledgerstate.Output {
	return ledgerstate.NewSigLockedColoredOutput(
		ledgerstate.NewColoredBalances(
			map[ledgerstate.Color]uint64{
				ledgerstate.ColorIOTA: balance,
			}),
		addr,
	)
}

// issueTX issues a transaction to the Tangle and waits for it to become booked.
func (s *StateManager) issueTX(tx *ledgerstate.Transaction) (msg *tangle.Message, err error) {
	// attach to message layer
	//issueTransaction := func() (*tangle.Message, error) {
	message, e := messagelayer.Tangle().IssuePayload(tx)
	if e != nil {
		return nil, e
	}
	return message, nil
}

// block for a certain amount of time until we know that the transaction
// actually got booked by this node itself
// TODO: replace with an actual more reactive way
//msg, err = messagelayer.AwaitMessageToBeBooked(issueTransaction, tx.ID(), s.maxTxBookedAwaitTime)
//if err != nil {
//	return nil, errors.Errorf("%w: tx %s", err, tx.ID().String())
//}
//return msg, nil
//}

// endregion

// region splittingEnv

type splittingEnv struct {
	// preparedTxID is a map that stores prepared and issued transaction IDs
	sync.RWMutex
	issuedTxIDs map[ledgerstate.TransactionID]types.Empty

	// channel to signal that listening has finished
	listeningFinished chan error

	// counts confirmed transactions during listening
	confirmedCount *atomic.Uint64

	// counts successful splits
	updateStateCount *atomic.Uint64
}

func newSplittingEnv() *splittingEnv {
	return &splittingEnv{
		issuedTxIDs:       make(map[ledgerstate.TransactionID]types.Empty),
		listeningFinished: make(chan error),
		confirmedCount:    atomic.NewUint64(0),
		updateStateCount:  atomic.NewUint64(0),
	}
}

func (s *splittingEnv) WasIssuedInThisPreparation(transactionID ledgerstate.TransactionID) bool {
	s.RLock()
	defer s.RUnlock()

	_, ok := s.issuedTxIDs[transactionID]
	return ok
}

func (s *splittingEnv) IssuedTransactionsCount() uint64 {
	s.RLock()
	defer s.RUnlock()

	return uint64(len(s.issuedTxIDs))
}

func (s *splittingEnv) IssuedTransactionsPrint() string {
	s.RLock()
	defer s.RUnlock()
	issuedStr := "IssuedTx = ["
	for tx := range s.issuedTxIDs {
		issuedStr += tx.Base58()
		issuedStr += ", "
	}
	issuedStr += "]\n"
	return issuedStr
}

func (s *splittingEnv) AddIssuedTxID(txID ledgerstate.TransactionID) {
	s.Lock()
	defer s.Unlock()
	s.issuedTxIDs[txID] = types.Void
}

// endregion

// region fulfilState
type fulfillState struct {
	// ordered list of available outputs to fund faucet requests
	fundingOutputs *list.List

	sync.RWMutex
}

func newFulfillState() *fulfillState {
	state := &fulfillState{
		fundingOutputs: list.New(),
	}

	return state
}

// FundingOutputsCount returns the number of available outputs that can be used to fund a request.
func (f *fulfillState) FundingOutputsCount() int {
	f.RLock()
	defer f.RUnlock()

	return f.fundingOutputsCount()
}

func (f *fulfillState) FundingOutputsAdd(fundingOutput *FaucetOutput) {
	f.Lock()
	defer f.Unlock()

	f.fundingOutputs.PushBack(fundingOutput)
}

// GetFundingOutput returns the first funding output in the list.
func (f *fulfillState) GetFundingOutput() (fundingOutput *FaucetOutput, err error) {
	f.Lock()
	defer f.Unlock()

	if f.fundingOutputsCount() < 1 {
		return nil, ErrNotEnoughFundingOutputs
	}
	fundingOutput = f.fundingOutputs.Remove(f.fundingOutputs.Front()).(*FaucetOutput)
	return
}

func (f *fulfillState) fundingOutputsCount() int {
	return f.fundingOutputs.Len()
}

// endregion

// region preparingState
type preparingState struct {
	// output that holds the remainder funds to the faucet, should always be on address 0
	remainderOutput *FaucetOutput
	// outputs that holds funds during the splitting period, filled in only with outputs needed for next split, should always be on address 1
	supplyOutputs *list.List
	// the last funding output address index, should start from MaxFaucetOutputsCount + 1
	// when we prepare new funding outputs, we start from lastFundingOutputAddressIndex + 1
	lastFundingOutputAddressIndex uint64
	// mapping base58 encoded addresses to their indices
	addressToIndex map[string]uint64
	// the seed instance of the faucet holding the tokens
	seed *walletseed.Seed
	// IsPreparingFunds indicates if faucet is currently preparing next batch of reminders
	IsPreparingFunds typeutils.AtomicBool

	// wgPreparing is used when fulfilling request for waiting for more funds in case they were not prepared on time
	sync.WaitGroup
	// ensures that fields related to new funds creation can be accesses by only one goroutine at the same time
	sync.RWMutex
}

func newPreparingState(seed *walletseed.Seed) *preparingState {
	state := &preparingState{
		seed: seed,
		addressToIndex: map[string]uint64{
			seed.Address(RemainderAddressIndex).Address().Base58(): RemainderAddressIndex,
		},
		lastFundingOutputAddressIndex: MaxFaucetOutputsCount + 1,
		supplyOutputs:                 list.New(),
		remainderOutput:               nil,
	}
	return state
}

func (p *preparingState) RemainderOutputBalance() uint64 {
	p.RLock()
	defer p.RUnlock()
	b := p.remainderOutput.Balance
	return b
}

func (p *preparingState) RemainderOutputID() ledgerstate.OutputID {
	p.RLock()
	defer p.RUnlock()
	id := p.remainderOutput.ID
	return id
}

func (p *preparingState) SetRemainderOutput(output *FaucetOutput) {
	p.Lock()
	defer p.Unlock()

	p.remainderOutput = output
}

// nextSupplyOutput returns the first supply address in the list.
func (p *preparingState) NextSupplyOutput() (supplyOutput *FaucetOutput, err error) {
	if p.supplyOutputsCount() < 1 {
		return nil, ErrNotEnoughSupplyOutputs
	}
	supplyOutput = p.supplyOutputs.Remove(p.supplyOutputs.Front()).(*FaucetOutput)
	return
}

// SupplyOutputsCount returns the number of available outputs that can be split to prepare more faucet funds.
func (p *preparingState) SupplyOutputsCount() int {
	p.RLock()
	defer p.RUnlock()

	return p.supplyOutputsCount()
}

func (p *preparingState) supplyOutputsCount() int {
	return p.supplyOutputs.Len()
}

func (p *preparingState) AddSupplyOutput(output *FaucetOutput) {
	p.Lock()
	defer p.Unlock()

	p.supplyOutputs.PushBack(output)
}

func (p *preparingState) IncrLastFundingOutputAddressIndex() uint64 {
	p.Lock()
	defer p.Unlock()

	p.lastFundingOutputAddressIndex++
	return p.lastFundingOutputAddressIndex
}

func (p *preparingState) GetLastFundingOutputAddressIndex() uint64 {
	p.RLock()
	defer p.RUnlock()

	return p.lastFundingOutputAddressIndex
}

// UpdateLastFundingOutputAddressIndex sets index if provided index is greater than the current one
func (p *preparingState) UpdateLastFundingOutputAddressIndex(index uint64) {
	p.Lock()
	defer p.Unlock()
	if index > p.lastFundingOutputAddressIndex {
		p.lastFundingOutputAddressIndex = index
	}
}

func (p *preparingState) GetAddressToIndex(addr string) uint64 {
	p.RLock()
	defer p.RUnlock()

	return p.addressToIndex[addr]
}

func (p *preparingState) AddAddressToIndex(addr string, index uint64) {
	p.Lock()
	defer p.Unlock()

	p.addressToIndex[addr] = index
}

// endregion

// region helper methods

type wallet struct {
	keyPair ed25519.KeyPair
}

func (w wallet) privateKey() ed25519.PrivateKey {
	return w.keyPair.PrivateKey
}

func (w wallet) publicKey() ed25519.PublicKey {
	return w.keyPair.PublicKey
}

func (w wallet) sign(txEssence *ledgerstate.TransactionEssence) *ledgerstate.ED25519Signature {
	return ledgerstate.NewED25519Signature(w.publicKey(), w.privateKey().Sign(txEssence.Bytes()))
}

// endregion
