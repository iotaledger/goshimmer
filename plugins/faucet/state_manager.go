package faucet

import (
	"container/list"
	"github.com/iotaledger/hive.go/typeutils"
	"sort"
	"sync"
	"time"

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

	// SupplyAddressIndex holds funds prepared for splitting, in case of failure during the split outputs will stay on this
	// address and will be joined in one bigger reminder output
	SupplyAddressIndex = 1

	// MinimumFaucetBalance defines the minimum token amount required, before the faucet stops operating.
	MinimumFaucetBalance = 0.1 * GenesisTokenAmount

	// MinimumFaucetFundsLeft defines the minimum fraction (x/100) of prepared fundingReminders limit that triggers funds preparation
	MinimumFaucetRemindersFractionLeft = 10

	// MaxFaucetOutputsCount defines the max outputs count for the Facuet as the ledgerstate.MaxOutputCount -1 remainder output.
	MaxFaucetOutputsCount = ledgerstate.MaxOutputCount - 1

	// WaitForConfirmation defines the wait time before considering a transaction confirmed.
	WaitForConfirmation = 10 * time.Second
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
	// the last funding output address index
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
			seed.Address(SupplyAddressIndex).Address().Base58():    SupplyAddressIndex,
		},

		supplyOutputs:        list.New(),
		tokensPerRequest:     tokensPerRequest,
		preparedOutputsCount: preparedOutputsCount,
		splittingMultiplayer: splittingMultiplayer,
		seed:                 seed,
		maxTxBookedAwaitTime: maxTxBookedTime,
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
//  - startIndex defines from which address index to start look for prepared outputs.
//  - remainder output should always sit on address 0.
//   - if no funding outputs are found, the faucet creates them from the remainder output.
func (s *StateManager) DeriveStateFromTangle(startIndex int) (err error) {
	// TODO check startIndex
	s.isPreparingFunds.Set()
	defer s.isPreparingFunds.UnSet()

	err = s.findUnspentRemainderOutput()
	if err != nil {
		return
	}
	endIndex := (GenesisTokenAmount-s.remainderOutput.Balance)/s.tokensPerRequest + 1
	Plugin().LogInfof("%d indices have already been used based on found remainder output", endIndex)

	foundPreparedOutputs := s.findPreparedOutputs(endIndex)

	if len(foundPreparedOutputs) == 0 {
		// prepare more funding outputs if we did not find any
		err = s.prepareMoreFundingOutputs()
		if err != nil {
			return errors.Errorf("Found no prepared outputs, failed to create them: %w", err)
		}
	} else {
		// else just save the found outputs into the state
		s.saveFundingOutputs(foundPreparedOutputs)
	}
	Plugin().LogInfof("Remainder output %s had %d funds", s.remainderOutput.ID.Base58(), s.remainderOutput.Balance)

	// check for any unfinished funds preparation and use all remaining supply outputs
	err = s.findSupplyOutputs()
	if err != nil {
		return
	}
	err = s.prepareSupplyFunding()

	return err
}

// FulFillFundingRequest fulfills a faucet request by spending the next funding output to the requested address.
// Mana of the transaction is pledged to the requesting node.
func (s *StateManager) FulFillFundingRequest(requestMsg *tangle.Message) (m *tangle.Message, txID string, err error) {

	faucetReq := requestMsg.Payload().(*faucet.Request)

	// trigger funds preparation, because there are less than 1/10th outputs left
	if uint64(s.FundingOutputsCount()) < s.splittingMultiplayer*s.preparedOutputsCount/MinimumFaucetRemindersFractionLeft {
		s.signalMoreFundingNeeded()
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

	return m, txID, err
}

// signalMoreFundingNeeded triggers preparation of faucet funding only if none preparation is currently running
func (s *StateManager) signalMoreFundingNeeded() {
	if s.isPreparingFunds.SetToIf(false, true) {
		//TODO add wait group or use some more sophisticated thingy to handle during the shutdown (should it wait for the confirmation?)
		go func() {
			Plugin().LogInfof("Preparing more outputs...")
			err := s.prepareMoreFundingOutputs()
			if err != nil {
				if errors.Is(err, ErrSplittingFundsFailed) {
					err = errors.Errorf("failed to prepare more outputs: %w", err)
					Plugin().LogError(err)
					return
				}
				if errors.Is(err, ErrConfirmationTimeoutExpired) {
					Plugin().LogInfof("Preparing more outputs partially successful: %w", err)
				}
				Plugin().LogInfof("Preparing more outputs... DONE")
			}
		}()
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

// saveFundingOutputs sorts the given slice of faucet funding outputs based on the address indices, and then saves them in stateManager.
func (s *StateManager) saveFundingOutputs(fundingOutputs []*FaucetOutput) {
	// sort prepared outputs based on address index
	sort.Slice(fundingOutputs, func(i, j int) bool {
		return fundingOutputs[i].AddressIndex < fundingOutputs[j].AddressIndex
	})

	s.fundingMutex.Lock()
	// fill prepared output list
	for _, fOutput := range fundingOutputs {
		s.fundingOutputs.PushBack(fOutput)
	}
	s.lastFundingOutputAddressIndex = s.fundingOutputs.Back().Value.(*FaucetOutput).AddressIndex
	s.fundingMutex.Unlock()

	Plugin().LogInfof("Added %d new funding outputs, last used address index is %d", len(fundingOutputs), s.lastFundingOutputAddressIndex)
	Plugin().LogInfof("There are currently %d prepared outputs in the faucet", s.FundingOutputsCount())
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

// findPreparedOutputs looks for prepared outputs in the tangle.
func (s *StateManager) findPreparedOutputs(endIndex uint64) []*FaucetOutput {

	foundPreparedOutputs := make([]*FaucetOutput, 0)

	Plugin().LogInfof("Looking for prepared outputs in the Tangle...")

	for i := startIndex; uint64(i) <= endIndex; i++ {
		messagelayer.Tangle().LedgerState.CachedOutputsOnAddress(s.seed.Address(uint64(i)).Address()).Consume(func(output ledgerstate.Output) {
			messagelayer.Tangle().LedgerState.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
				if outputMetadata.ConsumerCount() < 1 {
					iotaBalance, colorExist := output.Balances().Get(ledgerstate.ColorIOTA)
					if !colorExist {
						return
					}
					switch iotaBalance {
					case s.tokensPerRequest:
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

// findSupplyOutputs looks for splittingMultiplayer number of reminders of supply transaction and updates the StateManager
func (s *StateManager) findSupplyOutputs() (err error) {
	s.preparingMutex.Lock()
	defer s.preparingMutex.Unlock()

	var foundSupplyCount uint64

	supplyAddress := s.seed.Address(SupplyAddressIndex).Address()

	// remainder output should sit on address 1
	messagelayer.Tangle().LedgerState.CachedOutputsOnAddress(supplyAddress).Consume(func(output ledgerstate.Output) {
		if foundSupplyCount >= s.splittingMultiplayer {
			// return when enough outputs has been collected
			return
		}
		messagelayer.Tangle().LedgerState.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
			if outputMetadata.ConfirmedConsumer().Base58() == ledgerstate.GenesisTransactionID.Base58() &&
				outputMetadata.Finalized() {
				iotaBalance, ok := output.Balances().Get(ledgerstate.ColorIOTA)
				if !ok || iotaBalance != s.tokensPerRequest {
					return
				}
				supplyOutput := &FaucetOutput{
					ID:           output.ID(),
					Balance:      iotaBalance,
					Address:      output.Address(),
					AddressIndex: RemainderAddressIndex,
				}
				s.supplyOutputs.PushBack(supplyOutput)
				foundSupplyCount++
			}
		})
	})
	if foundSupplyCount == 0 {
		return errors.Errorf("can't find any supply output on address %s that has %d tokens", supplyAddress.Base58(), int(s.tokensPerRequest))
	}
	return
}

// nextSupplyReminder returns the first supply address in the list
func (s *StateManager) nextSupplyReminder() (supplyOutput *FaucetOutput, err error) {
	if s.supplyOutputs.Len() < 1 {
		return nil, ErrNotEnoughSupplyOutputs
	}
	supplyOutput = s.supplyOutputs.Remove(s.supplyOutputs.Front()).(*FaucetOutput)
	return
}

// prepareMoreFundingOutputs prepares more funding outputs by splitting up the remainder output and submits supply transactions
// SupplyAddressIndex
// submits the transaction
// to the Tangle, waits for its confirmation, and then updates the internal state of the faucet.
func (s *StateManager) prepareMoreFundingOutputs() (err error) {
	defer s.isPreparingFunds.UnSet()
	// no remainder output present
	err = s.findUnspentRemainderOutput()
	if err != nil {
		return errors.Errorf("%w: %w", ErrMissingRemainderOutput, err)
	}
	// if no error was returned, s.remainderOutput is not nil anymore

	if !s.isEnoughFunds() {
		err = ErrNotEnoughFunds
		return
	}

	err = s.prepareSupplyFunding()
	if err != nil {
		return errors.Errorf("%w: %w", ErrSupplyPreparationFailed, err)
	}

	err = s.findSupplyOutputs()
	if err != nil {
		return errors.Errorf("%w : %w", ErrMissingSupplyOutputs, err)
	}

	err = s.splitSupplyTransaction()
	if errors.Is(err, ErrSplittingFundsFailed) {
		return err
	}

	return
}

// isEnoughFunds indicates if there is enough funds to carry on the faucet funds preparation
func (s *StateManager) isEnoughFunds() bool {
	s.preparingMutex.Lock()
	defer s.preparingMutex.Unlock()
	// not enough funds to carry out operation
	if s.tokensPerRequest*s.preparedOutputsCount*s.splittingMultiplayer > s.remainderOutput.Balance {
		return false
	}
	return true
}

// prepareSupplyFunding take remainder output and split it up to create supply transaction that will be used for further splitting
func (s *StateManager) prepareSupplyFunding() (err error) {
	s.preparingMutex.Lock()
	defer s.preparingMutex.Unlock()

	tx, err := s.createSplittingTx(true)
	if err != nil {
		return err
	}
	m := make(map[ledgerstate.TransactionID]*ledgerstate.Transaction)
	m[tx.ID()] = tx
	err = s.waitUntilAndProcessAfterConfirmation(m)
	return
}

func (s *StateManager) splitSupplyTransaction() (err error) {
	s.preparingMutex.Lock()
	defer s.preparingMutex.Unlock()

	m := make(map[ledgerstate.TransactionID]*ledgerstate.Transaction)
	var tx *ledgerstate.Transaction
	for i := uint64(0); i < uint64(s.SupplyOutputsCount()); i++ {
		tx, err = s.createSplittingTx(false)
		if err != nil {
			return
		}
		m[tx.ID()] = tx
	}
	err = s.waitUntilAndProcessAfterConfirmation(m)
	return
}

// waitUntilAndProcessAfterConfirmation issues all transactions provided in preparedTxIDs,
// monitors them until confirmation and updates the faucet internal state
func (s *StateManager) waitUntilAndProcessAfterConfirmation(preparedTxIDs map[ledgerstate.TransactionID]*ledgerstate.Transaction) error {
	// buffered channel will store all confirmed transactions
	txConfirmed := make(chan ledgerstate.TransactionID, len(preparedTxIDs)) //s.preparedOutputsCount or 1

	monitorTxConfirmation := events.NewClosure(func(transactionID ledgerstate.TransactionID) {
		if _, ok := preparedTxIDs[transactionID]; ok {
			txConfirmed <- transactionID
		}
	})

	// listen on confirmation
	messagelayer.Tangle().LedgerState.UTXODAG.Events().TransactionConfirmed.Attach(monitorTxConfirmation)
	defer messagelayer.Tangle().LedgerState.UTXODAG.Events().TransactionConfirmed.Detach(monitorTxConfirmation)

	// issue all transactions from the map
	for txID, tx := range preparedTxIDs {
		// issue the tx
		_, issueErr := s.issueTX(tx)
		// remove tx from the map in cas of issuing failure
		if issueErr != nil {
			delete(preparedTxIDs, txID)
		}
	}

	// waiting for transactions to be confirmed
	ticker := time.NewTicker(WaitForConfirmation)
	defer ticker.Stop()
	timeoutCounter := 0
	maxWaitAttempts := 50 // 500 s max timeout (if fpc voting is in place)

	issuedCount := uint64(len(preparedTxIDs))
	var confirmedCount uint64
	var stateUpdatedCount uint64
	for {
		select {
		case confirmedTx := <-txConfirmed:
			confirmedCount += 1
			err := s.updateState(confirmedTx)
			if err == nil {
				stateUpdatedCount++
			}

			// all issued transactions has been confirmed
			if confirmedCount == issuedCount {
				return nil
			}
		case <-ticker.C:
			if timeoutCounter >= maxWaitAttempts {
				if confirmedCount == 0 {
					return ErrSplittingFundsFailed
				}
				return errors.Errorf("confirmed %d and saved %d out of %d issued transactions: %w", confirmedCount, stateUpdatedCount, issuedCount, ErrConfirmationTimeoutExpired)
			}
			timeoutCounter++
		}
	}

}

// updateState takes a confirmed transaction (splitting tx), and updates the faucet internal state based on its content.
func (s *StateManager) updateState(transactionID ledgerstate.TransactionID) (err error) {
	messagelayer.Tangle().LedgerState.Transaction(transactionID).Consume(func(transaction *ledgerstate.Transaction) {
		remainingBalance := s.remainderOutput.Balance - s.tokensPerRequest*s.preparedOutputsCount*s.splittingMultiplayer
		supplyBalance := s.tokensPerRequest * s.splittingMultiplayer

		fundingOutputs := make([]*FaucetOutput, 0, s.preparedOutputsCount)

		// derive information from outputs
		for _, output := range transaction.Essence().Outputs() {
			iotaBalance, hasIota := output.Balances().Get(ledgerstate.ColorIOTA)
			if !hasIota {
				err = errors.Errorf("tx outputs don't have IOTA balance ")
				return
			}
			switch iotaBalance {
			case s.tokensPerRequest:
				fundingOutputs = append(fundingOutputs, &FaucetOutput{
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
		// save the info in internal state
		s.saveFundingOutputs(fundingOutputs)
	})

	return err
}

// createSplittingTx takes the current remainder output and creates a first splitting transaction into s.preparedOutputsCount
// funding outputs and one remainder output if supplyTx is true. Otherwise it uses
func (s *StateManager) createSplittingTx(isSupplyTx bool) (*ledgerstate.Transaction, error) {
	var inputs ledgerstate.Inputs
	var outputs ledgerstate.Outputs
	var w wallet
	// prepare inputs and outputs for supply transaction
	if isSupplyTx {
		inputs = ledgerstate.NewInputs(ledgerstate.NewUTXOInput(s.remainderOutput.ID))
		// prepare s.preparedOutputsCount number of supply outputs for further splitting.
		outputs = make(ledgerstate.Outputs, 0, s.preparedOutputsCount+1)
		balance := s.tokensPerRequest * s.splittingMultiplayer
		// all funding outputs will land on supply address
		for i := uint64(0); i < s.preparedOutputsCount; i++ {
			outputs = append(outputs, s.createOutput(s.seed.Address(SupplyAddressIndex).Address(), balance))
		}
		// add the remainder output
		balance = s.remainderOutput.Balance - s.tokensPerRequest*s.splittingMultiplayer*s.preparedOutputsCount
		outputs = append(outputs, s.createOutput(s.seed.Address(RemainderAddressIndex).Address(), balance))
		// signature
		w = wallet{keyPair: *s.seed.KeyPair(RemainderAddressIndex)}

		// prepare inputs and outputs for splitting transaction
	} else {
		reminder, err := s.nextSupplyReminder()
		if err != nil {
			return nil, errors.Errorf("could not retrieve supply output: %w", err)
		}
		inputs = ledgerstate.NewInputs(ledgerstate.NewUTXOInput(reminder.ID))
		// prepare s.splittingMultiplayer number of funding outputs.
		outputs = make(ledgerstate.Outputs, 0, s.splittingMultiplayer)
		// start from the last used funding output address index
		for index := s.lastFundingOutputAddressIndex + 1; index < s.lastFundingOutputAddressIndex+1+s.splittingMultiplayer; index++ {
			outputs = append(outputs, s.createOutput(s.seed.Address(index).Address(), s.tokensPerRequest))
			s.addressToIndex[s.seed.Address(index).Address().Base58()] = index
		}

		// signature
		w = wallet{keyPair: *s.seed.KeyPair(SupplyAddressIndex)}

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
	issueTransaction := func() (*tangle.Message, error) {
		message, e := messagelayer.Tangle().IssuePayload(tx)
		if e != nil {
			return nil, e
		}
		return message, nil
	}

	// block for a certain amount of time until we know that the transaction
	// actually got booked by this node itself
	// TODO: replace with an actual more reactive way
	msg, err = messagelayer.AwaitMessageToBeBooked(issueTransaction, tx.ID(), s.maxTxBookedAwaitTime)
	if err != nil {
		return nil, errors.Errorf("%w: tx %s", err, tx.ID().String())
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
