package faucet

import (
	"container/list"
	"github.com/iotaledger/hive.go/types"
	"github.com/iotaledger/hive.go/workerpool"
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
	// ordered list of available outputs to fund faucet requests
	fundingOutputs *list.List
	// ensures that only one goroutine can change fundingOutputs map at the same time.
	fundingMutex sync.Mutex

	// output that holds the remainder funds to the faucet, should always be on address 0
	remainderOutput *FaucetOutput
	// outputs that holds funds during the splitting period, filled in only with outputs needed for next split, should always be on address 1
	supplyOutputs *list.List
	// the last funding output address index, should start from MaxFaucetOutputsCount + 1
	// when we prepare new funding outputs, we start from lastFundingOutputAddressIndex + 1
	lastFundingOutputAddressIndex uint64
	// mapping base58 encoded addresses to their indices
	addressToIndex map[string]uint64
	// ensures that fields related to new funds creation can be accesses by only one goroutine at the same time
	preparingMutex sync.Mutex

	// the amount of tokens to send to every request
	tokensPerRequest uint64
	// number of funding outputs to prepare for supply address that will be break down further if fundingOutputs is short on funds
	preparedOutputsCount uint64
	// number of funding outputs for each output in supply transaction during the splitting period
	splittingMultiplayer uint64
	// the seed instance of the faucet holding the tokens
	seed *walletseed.Seed

	// the time to await for the transaction fulfilling a funding request
	// to become booked in the value layer
	maxTxBookedAwaitTime time.Duration

	// isPreparingFunds indicates if faucet is currently preparing next batch of reminders
	isPreparingFunds typeutils.AtomicBool

	// wgPreparing is used when fulfilling request for waiting for more funds in case they were not prepared on time
	wgPreparing sync.WaitGroup

	// preparationEnv keeps all variables necessary to prepare more faucet funds
	preparationEnv *preparationEnv
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

	res := &StateManager{
		fundingOutputs: list.New(),
		addressToIndex: map[string]uint64{
			seed.Address(RemainderAddressIndex).Address().Base58(): RemainderAddressIndex,
		},
		lastFundingOutputAddressIndex: MaxFaucetOutputsCount + 1,
		supplyOutputs:                 list.New(),
		tokensPerRequest:              tokensPerRequest,
		preparedOutputsCount:          preparedOutputsCount,
		splittingMultiplayer:          splittingMultiplayer,
		seed:                          seed,
		maxTxBookedAwaitTime:          maxTxBookedTime,
	}

	return res
}

// FundingOutputsCount returns the number of available outputs that can be used to fund a request.
func (s *StateManager) FundingOutputsCount() int {
	s.fundingMutex.Lock()
	defer s.fundingMutex.Unlock()
	return s.fundingOutputs.Len()
}

// SupplyOutputsCount returns the number of available outputs that can be split to prepare more faucet funds.
func (s *StateManager) SupplyOutputsCount() int {
	return s.supplyOutputs.Len()
}

// DeriveStateFromTangle derives the faucet state from a synchronized Tangle.
//  - remainder output should always sit on address 0.
//  - supply outputs should be held on addresses 1-126
//  - faucet indexes stats from 127
//   - if no funding outputs are found, the faucet creates them from the remainder output.
func (s *StateManager) DeriveStateFromTangle() (err error) {
	s.isPreparingFunds.Set()
	defer s.isPreparingFunds.UnSet()

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

	endIndex := (GenesisTokenAmount-s.remainderOutput.Balance)/s.tokensPerRequest + 1
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

	Plugin().LogInfof("Added new funding outputs, last used address index is %d", s.lastFundingOutputAddressIndex)
	Plugin().LogInfof("There are currently %d prepared outputs in the faucet", s.FundingOutputsCount())
	Plugin().LogInfof("Remainder output %s had %d funds", s.remainderOutput.ID.Base58(), s.remainderOutput.Balance)

	return err
}

func (s *StateManager) printFaucetInternalState(msg string) {
	Plugin().LogInfof("State: %s is preparing funds %d, reminder index %d,\n funding len %d, supply len %d, last addr  used %d",
		msg, s.isPreparingFunds.IsSet(), s.remainderOutput.AddressIndex, s.FundingOutputsCount(), s.supplyOutputs.Len(), s.lastFundingOutputAddressIndex)
}

// FulFillFundingRequest fulfills a faucet request by spending the next funding output to the requested address.
// Mana of the transaction is pledged to the requesting node.
func (s *StateManager) FulFillFundingRequest(requestMsg *tangle.Message) (m *tangle.Message, txID string, err error) {
	faucetReq := requestMsg.Payload().(*faucet.Request)

	if s.notEnoughFundsInTheFaucet() {
		// wait if there is no outputs prepared
		waitForPreparation := s.FundingOutputsCount() == 0
		s.signalMoreFundingNeeded(waitForPreparation)
	}

	// get an output that we can spend
	fundingOutput, fErr := s.getFundingOutput()
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
	s.printFaucetInternalState("FulFillFundingRequest DONE")

	return
}

// notEnoughFundsInTheFaucet checks if number of funding outputs is lower than MinimumFaucetRemindersPercentageLeft of total funds prepared at once
func (s *StateManager) notEnoughFundsInTheFaucet() bool {
	return uint64(s.FundingOutputsCount()) < uint64(float64(s.splittingMultiplayer*s.preparedOutputsCount)*float64(MinimumFaucetRemindersPercentageLeft)/totalPercentage)
}

// signalMoreFundingNeeded triggers preparation of faucet funding only if none preparation is currently running
// if wait is true it awaits for funds to be prepared
func (s *StateManager) signalMoreFundingNeeded(wait bool) {
	if s.isPreparingFunds.SetToIf(false, true) {
		go func() {
			Plugin().LogInfof("Preparing more outputs...")
			err := s.prepareMoreFundingOutputs()
			_ = s.handlePrepareErrors(err)
		}()
	}
	// waits until preparation of funds will finish
	if wait {
		s.wgPreparing.Wait()
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

	w := wallet{keyPair: *s.seed.KeyPair(fundingOutput.AddressIndex)}
	unlockBlock := ledgerstate.NewSignatureUnlockBlock(w.sign(essence))

	tx = ledgerstate.NewTransaction(
		essence,
		ledgerstate.UnlockBlocks{unlockBlock},
	)
	return
}

// saveFundingOutputs saves the given slice of indices in StateManager and updates lastFundingOutputAddressIndex.
func (s *StateManager) saveFundingOutputs(fundingOutputs []*FaucetOutput) {
	s.fundingMutex.Lock()
	defer s.fundingMutex.Unlock()

	maxFundingOutputAddressIndex := s.lastFundingOutputAddressIndex
	// fill prepared output list
	for _, fOutput := range fundingOutputs {
		if maxFundingOutputAddressIndex < fOutput.AddressIndex {
			maxFundingOutputAddressIndex = fOutput.AddressIndex
		}
		s.fundingOutputs.PushBack(fOutput)
	}
	s.lastFundingOutputAddressIndex = maxFundingOutputAddressIndex
}

// getFundingOutput returns the first funding output in the list.
func (s *StateManager) getFundingOutput() (fundingOutput *FaucetOutput, err error) {
	s.fundingMutex.Lock()
	defer s.fundingMutex.Unlock()
	if s.fundingOutputs.Len() < 1 {
		return nil, ErrNotEnoughFundingOutputs
	}
	fundingOutput = s.fundingOutputs.Remove(s.fundingOutputs.Front()).(*FaucetOutput)
	return
}

// findFundingOutputs looks for prepared outputs in the tangle.
func (s *StateManager) findFundingOutputs(endIndex uint64) []*FaucetOutput {
	foundPreparedOutputs := make([]*FaucetOutput, 0)

	Plugin().LogInfof("Looking for prepared outputs in the Tangle...")

	for i := MaxFaucetOutputsCount + 1; uint64(i) <= endIndex; i++ {
		messagelayer.Tangle().LedgerState.CachedOutputsOnAddress(s.seed.Address(uint64(i)).Address()).Consume(func(output ledgerstate.Output) {
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
	s.preparingMutex.Lock()
	defer s.preparingMutex.Unlock()

	var foundRemainderOutput *FaucetOutput

	remainderAddress := s.seed.Address(RemainderAddressIndex).Address()

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
	s.remainderOutput = foundRemainderOutput

	return nil
}

// findSupplyOutputs looks for preparedOutputsCount number of reminders of supply transaction and updates the StateManager
func (s *StateManager) findSupplyOutputs() (err error) {
	s.preparingMutex.Lock()
	defer s.preparingMutex.Unlock()

	var foundSupplyCount uint64
	var foundOnCurrentAddress bool

	// supply outputs should sit on addresses 1-126
	for supplyAddr := uint64(1); supplyAddr < MaxFaucetOutputsCount+1; supplyAddr++ {
		supplyAddress := s.seed.Address(supplyAddr).Address()
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
					s.supplyOutputs.PushBack(supplyOutput)
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

// nextSupplyReminder returns the first supply address in the list.
func (s *StateManager) nextSupplyReminder() (supplyOutput *FaucetOutput, err error) {
	if s.supplyOutputs.Len() < 1 {
		return nil, ErrNotEnoughSupplyOutputs
	}
	supplyOutput = s.supplyOutputs.Remove(s.supplyOutputs.Front()).(*FaucetOutput)
	return
}

// prepareMoreFundingOutputs prepares more funding outputs by splitting up the remainder output on preparedOutputsCount outputs plus new reminder.
// and submits supply transactions. After transaction is confirmed it uses each supply output and splits it again for splittingMultiplayer many times.
// After confirmation, outputs are added to fundingOutputs list.
// Reminder is stored on address 0. Next 126 indexes are reserved for supply transaction outputs.
func (s *StateManager) prepareMoreFundingOutputs() (err error) {
	s.wgPreparing.Add(1)
	defer s.wgPreparing.Done()

	defer s.isPreparingFunds.UnSet()
	// no remainder output present
	err = s.findUnspentRemainderOutput()
	if err != nil {
		return errors.Errorf("%w: %w", ErrMissingRemainderOutput, err)
	}
	// if no error was returned, s.remainderOutput is not nil anymore

	if s.notEnoughFunds() {
		err = ErrNotEnoughFunds
		return
	}

	err = s.prepareSupplyFunding()
	if err != nil {
		return errors.Errorf("%w: %w", ErrSupplyPreparationFailed, err)
	}
	s.printFaucetInternalState("after prepareSupplyFunding")
	err = s.splitSupplyTransaction()
	if errors.Is(err, ErrSplittingFundsFailed) {
		return err
	}
	s.printFaucetInternalState("after splitSupplyTransaction")

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
	s.preparingMutex.Lock()
	defer s.preparingMutex.Unlock()
	// not enough funds to carry out operation
	return s.remainderOutput.Balance < s.tokensPerRequest*s.preparedOutputsCount*s.splittingMultiplayer
}

// prepareSupplyFunding takes a remainder output and splits it up to create supply transaction that will be used for further splitting
func (s *StateManager) prepareSupplyFunding() (err error) {
	preparationFailure := make(chan types.Empty)
	s.preparationEnv = newPreparationEnv()
	Plugin().LogInfof("Created new preparation environment for supply")

	go s.preparationEnv.listenOnConfirmationAndUpdateState(s, 1, preparationFailure)
	_, ok := preparingWorkerPool.TrySubmit(s.supplyTransactionElements, preparationFailure)
	if !ok {
		Plugin().LogInfo("supply funding task not submitted, queue is full")
	}
	// wait for listenOnConfirmationAndUpdateState to return
	err = <-s.preparationEnv.listeningFinished
	s.printFaucetInternalState("Finished listening on supply AAA")

	return
}

// prepareTransactionTask function for preparation workerPool that uses provided callback function
// to create either supply or split transaction
func (s *StateManager) prepareTransactionTask(task workerpool.Task) {
	transactionElementsCallback := task.Param(0).(func() (inputs ledgerstate.Inputs, outputs ledgerstate.Outputs, w wallet, err error))
	//wg := task.Param(1).(*sync.WaitGroup)
	preparationFailed := task.Param(1).(chan types.Empty)

	tx, err := s.createSplittingTx(transactionElementsCallback)
	if err != nil {
		preparationFailed <- types.Void
		return
	}
	s.preparationEnv.issuePreparedTransaction(s, tx)
	return
}

// splitSupplyTransaction splits further outputs from supply transaction to create fundingReminders.
// It listens for transaction confirmation and submits transaction preparation and issuance to the worker pool.
func (s *StateManager) splitSupplyTransaction() (err error) {
	preparationFailure := make(chan types.Empty)

	supplyToProcess := uint64(s.SupplyOutputsCount())
	s.preparationEnv = newPreparationEnv()
	go s.preparationEnv.listenOnConfirmationAndUpdateState(s, supplyToProcess, preparationFailure)

	for i := uint64(0); i < supplyToProcess; i++ {
		preparingWorkerPool.TrySubmit(s.splittingTransactionElements, preparationFailure)
	}

	// wait for listenOnConfirmationAndUpdateState to return
	err = <-s.preparationEnv.listeningFinished
	return
}

// listenOnConfirmationAndUpdateState submits splitting transactions to the worker pool,
// listen for the confirmation and updates the faucet internal state
func (p *preparationEnv) listenOnConfirmationAndUpdateState(manager *StateManager, txNumToProcess uint64, preparationFailure <-chan types.Empty) {
	Plugin().LogInfof("Start listening for confirmation")
	// buffered channel will store all confirmed transactions
	txConfirmed := make(chan ledgerstate.TransactionID, txNumToProcess) // length is s.preparedOutputsCount or 1

	monitorTxConfirmation := events.NewClosure(func(transactionID ledgerstate.TransactionID) {
		if p.wasIssuedInThisPreparation(transactionID) {
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

	p.confirmedCount = 0
	for {
		select {
		case confirmedTx := <-txConfirmed:
			finished := p.onConfirmation(manager, confirmedTx, issuedCount)
			if finished {
				p.listeningFinished <- nil
				return
			}
		case <-ticker.C:
			err, finished := p.onTickerCheckMaxAttempts(timeoutCounter, maxWaitAttempts, issuedCount)
			if finished {
				p.listeningFinished <- err
				return
			}
		case <-preparationFailure:
			// issued count after all transaction were issued
			Plugin().LogInfof("listenOnConfirmationAndUpdateState issuedCount decreased %d", issuedCount-1)
			issuedCount--
		}
	}
}

func (p *preparationEnv) onTickerCheckMaxAttempts(timeoutCounter int, maxWaitAttempts int, issuedCount uint64) (err error, finished bool) {
	if timeoutCounter >= maxWaitAttempts {
		if p.confirmedCount == 0 {
			err = ErrSplittingFundsFailed
			return err, true
		}
		return errors.Errorf("confirmed %d and saved %d out of %d issued transactions: %w", p.confirmedCount, p.updateStateCount, issuedCount, ErrConfirmationTimeoutExpired), true
	}
	timeoutCounter++
	Plugin().LogInfof("Tick")
	return nil, false
}

func (p *preparationEnv) onConfirmation(manager *StateManager, confirmedTx ledgerstate.TransactionID, issuedCount uint64) (finished bool) {
	p.confirmedCount++
	err := manager.updateState(confirmedTx)
	if err == nil {
		p.updateStateCount++
	}
	// all issued transactions has been confirmed
	if p.confirmedCount == issuedCount {
		Plugin().LogInfof("Listening on prepared transaction finished,  %d transactions")
		return true
	}
	return false
}

// updateState takes a confirmed transaction (splitting tx), and updates the faucet internal state based on its content.
func (s *StateManager) updateState(transactionID ledgerstate.TransactionID) (err error) {
	messagelayer.Tangle().LedgerState.Transaction(transactionID).Consume(func(transaction *ledgerstate.Transaction) {
		remainingBalance := s.remainderOutput.Balance - s.tokensPerRequest*s.preparedOutputsCount*s.splittingMultiplayer
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
				s.fundingOutputs.PushBack(&FaucetOutput{
					ID:           output.ID(),
					Balance:      iotaBalance,
					Address:      output.Address(),
					AddressIndex: s.addressToIndex[output.Address().Base58()],
				})
			case remainingBalance:
				s.remainderOutput = &FaucetOutput{
					ID:           output.ID(),
					Balance:      iotaBalance,
					Address:      output.Address(),
					AddressIndex: s.addressToIndex[output.Address().Base58()],
				}
			case supplyBalance:
				s.supplyOutputs.PushBack(&FaucetOutput{
					ID:           output.ID(),
					Balance:      iotaBalance,
					Address:      output.Address(),
					AddressIndex: s.addressToIndex[output.Address().Base58()],
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
	s.preparingMutex.Lock()
	defer s.preparingMutex.Unlock()

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
	inputs = ledgerstate.NewInputs(ledgerstate.NewUTXOInput(s.remainderOutput.ID))
	// prepare s.preparedOutputsCount number of supply outputs for further splitting.
	outputs = make(ledgerstate.Outputs, 0, s.preparedOutputsCount+1)
	balance := s.tokensPerRequest * s.splittingMultiplayer
	// all funding outputs will land on supply addresses 1 to 126
	for index := uint64(1); index < s.preparedOutputsCount+1; index++ {
		outputs = append(outputs, s.createOutput(s.seed.Address(index).Address(), balance))
		s.addressToIndex[s.seed.Address(index).Address().Base58()] = index
	}
	// add the remainder output
	balance = s.remainderOutput.Balance - s.tokensPerRequest*s.splittingMultiplayer*s.preparedOutputsCount
	outputs = append(outputs, s.createOutput(s.seed.Address(RemainderAddressIndex).Address(), balance))
	// signature
	w = wallet{keyPair: *s.seed.KeyPair(RemainderAddressIndex)}
	return
}

// splittingTransactionElements is a callback function used during creation of splitting transactions.
// It splits each supply output into funding remainders and uses lastFundingOutputAddressIndex to derive their address.
func (s *StateManager) splittingTransactionElements() (inputs ledgerstate.Inputs, outputs ledgerstate.Outputs, w wallet, err error) {
	reminder, err := s.nextSupplyReminder()
	if err != nil {
		err = errors.Errorf("could not retrieve supply output: %w", err)
		return
	}
	inputs = ledgerstate.NewInputs(ledgerstate.NewUTXOInput(reminder.ID))
	// prepare s.splittingMultiplayer number of funding outputs.
	outputs = make(ledgerstate.Outputs, 0, s.splittingMultiplayer)
	// start from the last used funding output address index
	for i := uint64(0); i < s.splittingMultiplayer; i++ {
		s.lastFundingOutputAddressIndex++
		addr := s.seed.Address(s.lastFundingOutputAddressIndex).Address()
		outputs = append(outputs, s.createOutput(addr, s.tokensPerRequest))
		s.addressToIndex[addr.Base58()] = s.lastFundingOutputAddressIndex
	}
	// signature
	w = wallet{keyPair: *s.seed.KeyPair(reminder.AddressIndex)}

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
	//}
	//Plugin().LogInfof("issueTx before await")
	// block for a certain amount of time until we know that the transaction
	// actually got booked by this node itself
	// TODO: replace with an actual more reactive way
	//msg, err = messagelayer.AwaitMessageToBeBooked(issueTransaction, tx.ID(), s.maxTxBookedAwaitTime)
	//if err != nil {
	//	return nil, errors.Errorf("%w: tx %s", err, tx.ID().String())
	//}
	//return
}

// endregion

// region preparationEnv

type preparationEnv struct {
	// workerPool for funds preparation
	//workerPool *workerpool.NonBlockingQueuedWorkerPool

	// preparedTxID is a map that stores prepared and issued transaction IDs
	issuedTxIDs map[ledgerstate.TransactionID]types.Empty

	// RWMutex for preparedTxID map
	preparationMutex sync.RWMutex

	// channel to signal that listening has finished
	listeningFinished chan error

	// counts confirmed transactions during listening
	confirmedCount uint64

	// counts successful splits
	updateStateCount uint64
}

func newPreparationEnv() *preparationEnv {
	return &preparationEnv{
		//workerPool:  workerpool.NewNonBlockingQueuedWorkerPool(workerPoolTask, workerpool.WorkerCount(preparationWorkerCount), workerpool.QueueSize(preparationWorkerQueueSize)),
		issuedTxIDs:       make(map[ledgerstate.TransactionID]types.Empty),
		listeningFinished: make(chan error),
	}
}

func (p *preparationEnv) wasIssuedInThisPreparation(transactionID ledgerstate.TransactionID) bool {
	p.preparationMutex.RLock()
	defer p.preparationMutex.RUnlock()
	_, ok := p.issuedTxIDs[transactionID]
	return ok
}

func (p *preparationEnv) issuedTransactionsCount() uint64 {
	p.preparationMutex.RLock()
	defer p.preparationMutex.RUnlock()

	return uint64(len(p.issuedTxIDs))
}

func (p *preparationEnv) issuePreparedTransaction(manager *StateManager, tx *ledgerstate.Transaction) {
	p.preparationMutex.Lock()
	defer p.preparationMutex.Unlock()
	_, err := manager.issueTX(tx)
	p.issuedTxIDs[tx.ID()] = types.Void
	if err != nil {
		return
	}
	return
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
