package faucet

import (
	"container/list"
	"sort"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"golang.org/x/xerrors"

	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/clock"
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
	// output that holds the remainder funds to the faucet, should always be on address 0
	remainderOutput *FaucetOutput
	// the last funding output address index
	// when we prepare new funding outputs, we start from lastFundingOutputAddressIndex + 1
	lastFundingOutputAddressIndex uint64
	// mapping base58 encoded addresses to their indices
	addressToIndex map[string]uint64

	// the amount of tokens to send to every request
	tokensPerRequest uint64
	// number of funding outputs to prepare if fundingOutputs is exhausted
	preparedOutputsCount uint64
	// the seed instance of the faucet holding the tokens
	seed *walletseed.Seed

	// the time to await for the transaction fulfilling a funding request
	// to become booked in the value layer
	maxTxBookedAwaitTime time.Duration

	// ensures that only one goroutine can work on the stateManager at any time
	sync.RWMutex
}

// NewStateManager creates a new state manager for the faucet.
func NewStateManager(
	tokensPerRequest uint64,
	seed *walletseed.Seed,
	preparedOutputsCount uint64,
	maxTxBookedTime time.Duration,
) *StateManager {
	// currently the max number of outputs in a tx is 127, therefore, when creating the splitting tx, we can have at most
	// 126 prepared outputs (+1 remainder output).
	// TODO: break down the splitting into more tx steps to be able to create more, than 126
	if preparedOutputsCount > MaxFaucetOutputsCount {
		preparedOutputsCount = MaxFaucetOutputsCount
	}
	res := &StateManager{
		fundingOutputs: list.New(),
		addressToIndex: map[string]uint64{
			seed.Address(RemainderAddressIndex).Address().Base58(): RemainderAddressIndex,
		},

		tokensPerRequest:     tokensPerRequest,
		preparedOutputsCount: preparedOutputsCount,
		seed:                 seed,
		maxTxBookedAwaitTime: maxTxBookedTime,
	}

	return res
}

// FundingOutputsCount returns the number of available outputs that can be used to fund a request.
func (s *StateManager) FundingOutputsCount() int {
	s.RLock()
	defer s.RUnlock()
	return s.fundingOutputs.Len()
}

// DeriveStateFromTangle derives the faucet state from a synchronized Tangle.
//  - startIndex defines from which address index to start look for prepared outputs.
//  - remainder output should always sit on address 0.
//   - if no funding outputs are found, the faucet creates them from the remainder output.
func (s *StateManager) DeriveStateFromTangle(startIndex int) (err error) {
	s.Lock()
	defer s.Unlock()

	foundPreparedOutputs := make([]*FaucetOutput, 0)
	toBeSweptOutputs := make([]*FaucetOutput, 0)
	var foundRemainderOutput *FaucetOutput

	remainderAddress := s.seed.Address(RemainderAddressIndex).Address()

	// remainder output should sit on address 0
	messagelayer.Tangle().LedgerState.OutputsOnAddress(remainderAddress).Consume(func(output ledgerstate.Output) {
		messagelayer.Tangle().LedgerState.OutputMetadata(output.ID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
			if outputMetadata.ConsumerCount() < 1 {
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
		return xerrors.Errorf("can't find an output on address %s that has at least %d tokens", remainderAddress.Base58(), int(MinimumFaucetBalance))
	}

	endIndex := (GenesisTokenAmount - foundRemainderOutput.Balance) / s.tokensPerRequest
	log.Infof("%d indices have already been used based on found remainder output", endIndex)

	log.Infof("Looking for prepared outputs in the Tangle...")

	for i := startIndex; uint64(i) <= endIndex; i++ {
		messagelayer.Tangle().LedgerState.OutputsOnAddress(s.seed.Address(uint64(i)).Address()).Consume(func(output ledgerstate.Output) {
			messagelayer.Tangle().LedgerState.OutputMetadata(output.ID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
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
					default:
						toBeSweptOutputs = append(toBeSweptOutputs, &FaucetOutput{
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
	log.Infof("Found %d prepared outputs in the Tangle", len(foundPreparedOutputs))
	log.Infof("Looking for prepared outputs in the Tangle... DONE")

	s.remainderOutput = foundRemainderOutput

	if len(foundPreparedOutputs) == 0 {
		// prepare more funding outputs if we did not find any
		err = s.prepareMoreFundingOutputs()
		if err != nil {
			return xerrors.Errorf("Found no prepared outputs, failed to create them: %w", err)
		}
	} else {
		// else just save the found outputs into the state
		s.saveFundingOutputs(foundPreparedOutputs)
	}
	log.Infof("Remainder output %s had %d funds", s.remainderOutput.ID.Base58(), s.remainderOutput.Balance)
	// ignore toBeSweptOutputs
	return err
}

// FulFillFundingRequest fulfills a faucet request by spending the next funding output to the requested address.
// Mana of the transaction is pledged to the requesting node.
func (s *StateManager) FulFillFundingRequest(requestMsg *tangle.Message) (m *tangle.Message, txID string, err error) {
	s.Lock()
	defer s.Unlock()

	faucetReq := requestMsg.Payload().(*Request)

	// get an output that we can spend
	fundingOutput, fErr := s.getFundingOutput()
	// we don't have funding outputs
	if xerrors.Is(fErr, ErrNotEnoughFundingOutputs) {
		// try preparing them
		log.Infof("Preparing more outputs...")
		pErr := s.prepareMoreFundingOutputs()
		if pErr != nil {
			err = xerrors.Errorf("failed to prepare more outputs: %w", pErr)
			return
		}
		log.Infof("Preparing more outputs... DONE")
		// and try getting the output again
		fundingOutput, fErr = s.getFundingOutput()
		if fErr != nil {
			err = xerrors.Errorf("failed to gather funding outputs")
			return
		}
	} else if fErr != nil {
		err = xerrors.Errorf("failed to gather funding outputs")
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

	// fill prepared output list
	for _, fOutput := range fundingOutputs {
		s.fundingOutputs.PushBack(fOutput)
	}
	s.lastFundingOutputAddressIndex = s.fundingOutputs.Back().Value.(*FaucetOutput).AddressIndex

	log.Infof("Added %d new funding outputs, last used address index is %d", len(fundingOutputs), s.lastFundingOutputAddressIndex)
	log.Infof("There are currently %d prepared outputs in the faucet", s.fundingOutputs.Len())
}

// getFundingOutput returns the first funding output in the list.
func (s *StateManager) getFundingOutput() (fundingOutput *FaucetOutput, err error) {
	if s.fundingOutputs.Len() < 1 {
		return nil, ErrNotEnoughFundingOutputs
	}
	fundingOutput = s.fundingOutputs.Remove(s.fundingOutputs.Front()).(*FaucetOutput)
	return
}

// prepareMoreFundingOutputs prepares more funding outputs by splitting up the remainder output, submits the transaction
// to the Tangle, waits for its confirmation, and then updates the internal state of the faucet.
func (s *StateManager) prepareMoreFundingOutputs() (err error) {
	// no remainder output present
	if s.remainderOutput == nil {
		err = ErrMissingRemainderOutput
		return
	}

	// not enough funds to carry out operation
	if s.tokensPerRequest*s.preparedOutputsCount > s.remainderOutput.Balance {
		err = ErrNotEnoughFunds
		return
	}

	// take remainder output and split it up
	tx := s.createSplittingTx()

	txConfirmed := make(chan *ledgerstate.Transaction, 1)

	monitorTxConfirmation := events.NewClosure(
		func(msgID tangle.MessageID) {
			messagelayer.Tangle().Storage.Message(msgID).Consume(func(msg *tangle.Message) {
				if msg.Payload().Type() == ledgerstate.TransactionType {
					msgTx, _, er := ledgerstate.TransactionFromBytes(msg.Payload().Bytes())
					if er != nil {
						// log.Errorf("Message %s contains invalid transaction payload: %w", msgID.String(), err)
						return
					}
					if msgTx.ID() == tx.ID() {
						txConfirmed <- msgTx
					}
				}
			})
		})

	// listen on confirmation
	messagelayer.Tangle().ConsensusManager.Events.TransactionConfirmed.Attach(monitorTxConfirmation)
	defer messagelayer.Tangle().ConsensusManager.Events.TransactionConfirmed.Detach(monitorTxConfirmation)

	// issue the tx
	issuedMsg, issueErr := s.issueTX(tx)
	if issueErr != nil {
		return issueErr
	}

	ticker := time.NewTicker(WaitForConfirmation)
	defer ticker.Stop()
	timeoutCounter := 0
	maxWaitAttempts := 10 // 100 s max timeout (if fpc voting is in place)

	for {
		select {
		case confirmedTx := <-txConfirmed:
			err = s.updateState(confirmedTx)
			return err
		case <-ticker.C:
			if timeoutCounter >= maxWaitAttempts {
				return xerrors.Errorf("Message %s: %w", issuedMsg.ID(), ErrConfirmationTimeoutExpired)
			}
			timeoutCounter++
		}
	}
}

// updateState takes a confirmed transaction (splitting tx), and updates the faucet internal state based on its content.
func (s *StateManager) updateState(tx *ledgerstate.Transaction) error {
	remainingBalance := s.remainderOutput.Balance - s.tokensPerRequest*s.preparedOutputsCount
	fundingOutputs := make([]*FaucetOutput, 0, s.preparedOutputsCount)

	// derive information from outputs
	for _, output := range tx.Essence().Outputs() {
		iotaBalance, hasIota := output.Balances().Get(ledgerstate.ColorIOTA)
		if !hasIota {
			return xerrors.Errorf("tx outputs don't have IOTA balance ")
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
		default:
			err := xerrors.Errorf("tx %s should not have output with balance %d", tx.ID().Base58(), iotaBalance)
			return err
		}
	}
	// save the info in internal state
	s.saveFundingOutputs(fundingOutputs)
	return nil
}

// createSplittingTx takes the current remainder output and creates a transaction that splits it into s.preparedOutputsCount
// funding outputs and one remainder output.
func (s *StateManager) createSplittingTx() *ledgerstate.Transaction {
	inputs := ledgerstate.NewInputs(ledgerstate.NewUTXOInput(s.remainderOutput.ID))

	// prepare s.preparedOutputsCount number of funding outputs.
	outputs := make(ledgerstate.Outputs, 0, s.preparedOutputsCount+1)
	// start from the last used funding output address index
	for i := s.lastFundingOutputAddressIndex + 1; i < s.lastFundingOutputAddressIndex+1+s.preparedOutputsCount; i++ {
		outputs = append(outputs, ledgerstate.NewSigLockedColoredOutput(
			ledgerstate.NewColoredBalances(
				map[ledgerstate.Color]uint64{
					ledgerstate.ColorIOTA: s.tokensPerRequest,
				}),
			s.seed.Address(i).Address(),
		),
		)
		s.addressToIndex[s.seed.Address(i).Address().Base58()] = i
	}

	// add the remainder output
	outputs = append(outputs, ledgerstate.NewSigLockedColoredOutput(
		ledgerstate.NewColoredBalances(
			map[ledgerstate.Color]uint64{
				ledgerstate.ColorIOTA: s.remainderOutput.Balance - s.tokensPerRequest*s.preparedOutputsCount,
			}),

		s.seed.Address(RemainderAddressIndex).Address(),
	),
	)

	essence := ledgerstate.NewTransactionEssence(
		0,
		clock.SyncedTime(),
		local.GetInstance().ID(),
		// consensus mana is pledged to EmptyNodeID
		identity.ID{},
		ledgerstate.NewInputs(inputs...),
		ledgerstate.NewOutputs(outputs...),
	)

	w := wallet{keyPair: *s.seed.KeyPair(s.remainderOutput.AddressIndex)}
	unlockBlock := ledgerstate.NewSignatureUnlockBlock(w.sign(essence))

	tx := ledgerstate.NewTransaction(
		essence,
		ledgerstate.UnlockBlocks{unlockBlock},
	)
	return tx
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
		return nil, xerrors.Errorf("%w: tx %s", err, tx.ID().String())
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
