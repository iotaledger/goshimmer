package evilwallet

import (
	"errors"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/plugins/faucet"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/identity"
)

const (
	GoFConfirmed             = 3
	waitForConfirmation      = 60 * time.Second
	FaucetRequestSplitNumber = 100

	maxGoroutines = 5
)

var clientsURL = []string{"http://localhost:8080", "http://localhost:8090"}

// region EvilWallet ///////////////////////////////////////////////////////////////////////////////////////////////////////

// EvilWallet provides a user-friendly way to do complicated double spend scenarios.
type EvilWallet struct {
	wallets         *Wallets
	connector       Clients
	outputManager   *OutputManager
	conflictManager *ConflictManager
	aliasManager    *AliasManager
}

// NewEvilWallet creates an EvilWallet instance.
func NewEvilWallet() *EvilWallet {
	connector := NewConnector(clientsURL)

	return &EvilWallet{
		wallets:         NewWallets(),
		connector:       connector,
		outputManager:   NewOutputManager(connector),
		conflictManager: NewConflictManager(),
		aliasManager:    NewAliasManager(),
	}
}

// NewWallet creates a new wallet of theh given wallet type.
func (e *EvilWallet) NewWallet(wType WalletType) *Wallet {
	return e.wallets.NewWallet(wType)
}

// GetClients returns the given number of clients.
func (e *EvilWallet) GetClients(num int) []*client.GoShimmerAPI {
	return e.connector.GetClients(num)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// EvilWallet Faucet Requests /////////////////////////////////////////////////////////////////////////////////////////////////////

// RequestFundsFromFaucet requests funds from the faucet, then track the confirmed status of unspent output,
// also register the alias name for the unspent output if provided.
func (e *EvilWallet) RequestFundsFromFaucet(address string, options ...FaucetRequestOption) (err error) {
	buildOptions := NewFaucetRequestOptions(options...)

	// request funds from faucet
	err = e.connector.SendFaucetRequest(address)
	if err != nil {
		return
	}

	// track output in output manager and make sure it's confirmed
	outputIDs := e.outputManager.AddOutputsByAddress(address)
	if len(outputIDs) == 0 {
		err = errors.New("no outputIDs found on address ")
		return
	}

	allConfirmed := e.outputManager.Track(outputIDs)
	if !allConfirmed {
		err = errors.New("output not confirmed")
		return
	}

	if len(buildOptions.aliasName) > 0 {
		input := ledgerstate.NewUTXOInput(outputIDs[0])
		err = e.aliasManager.AddInputAlias(input, "1")
	}

	return
}

// CreateNFreshFaucet10kWallet creates n new wallets, each wallet is created from one faucet request.
func (e *EvilWallet) CreateNFreshFaucet10kWallet(numberOf10kWallets int) {
	// channel to block the number of concurrent goroutines
	semaphore := make(chan bool, maxGoroutines)
	wg := sync.WaitGroup{}

	for reqNum := 0; reqNum < numberOf10kWallets; reqNum++ {
		wg.Add(1)
		go func(reqNum int) {
			defer wg.Done()
			// block and release goroutines
			semaphore <- true
			defer func() {
				<-semaphore
			}()
			err := e.CreateFreshFaucetWallet()
			if err != nil {
				return
			}
		}(reqNum)
	}
	wg.Wait()
	return
}

// CreateFreshFaucetWallet creates a new wallet and fills the wallet with 10000 outputs created from funds
// requested from the Faucet.
func (e *EvilWallet) CreateFreshFaucetWallet() (err error) {
	funds, err := e.requestAndSplitFaucetFunds()
	if err != nil {
		return
	}
	w := e.NewWallet(fresh)
	txIDs := e.splitOutputs(funds, w, FaucetRequestSplitNumber)

	e.outputManager.AwaitTransactionsConfirmationAndUpdateWallet(w, txIDs, maxGoroutines)
	return
}

func (e *EvilWallet) requestAndSplitFaucetFunds() (wallet *Wallet, err error) {
	reqWallet := NewWallet(fresh)
	addr := reqWallet.Address()
	initOutput := e.requestFaucetFunds(addr.Address())
	if err != nil {
		return
	}
	reqWallet.AddUnspentOutput(addr.Address(), initOutput, uint64(faucet.Parameters.TokensPerRequest))
	//first split 1 to FaucetRequestSplitNumber outputs
	wallet = NewWallet(fresh)
	e.outputManager.AwaitWalletOutputsToBeConfirmed(reqWallet)
	txIDs := e.splitOutputs(reqWallet, wallet, FaucetRequestSplitNumber)
	e.outputManager.AwaitTransactionsConfirmationAndUpdateWallet(wallet, txIDs, maxGoroutines)
	return
}

func (e *EvilWallet) requestFaucetFunds(addr ledgerstate.Address) (outputID ledgerstate.OutputID) {
	err := e.connector.SendFaucetRequest(addr.Base58())
	if err != nil {
		return
	}
	outputIDs := e.outputManager.AddOutputsByAddress(addr.Base58())
	ok := e.outputManager.Track(outputIDs)
	if !ok {
		return
	}
	outputID = outputIDs[0]
	return
}

func (e *EvilWallet) splitOutputs(inputWallet *Wallet, outputWallet *Wallet, splitNumber int) []string {
	wg := sync.WaitGroup{}

	txIDs := make([]string, len(inputWallet.unspentOutputs))
	if inputWallet.unspentOutputs == nil {
		return []string{}
	}
	inputNum := 0
	for _, output := range inputWallet.unspentOutputs {
		wg.Add(1)
		go func(inputNum int, out *Output) {
			defer wg.Done()
			// todo finish with a new create txs
			//tx := prepareTransaction([]*UnspentOutput{out}, []*seed.Seed{inputWallet.seed}, outWallet, numOutputs, *pledgeID)
			//clt := e.connector.GetClient()
			//txID, err := e.connector.PostTransaction(tx.Bytes(), clt)
			//if err != nil {
			//	return
			//}
			//txIDs[inputNum] = txID.TransactionID
		}(inputNum, output)
		inputNum++
	}
	wg.Wait()
	return txIDs
}

// CreateAlias registers an aliasName for the given data.
func (e *EvilWallet) CreateAlias(aliasName string, value interface{}) (err error) {
	switch t := value.(type) {
	case ledgerstate.Output:
		err = e.aliasManager.AddOutputAlias(t, aliasName)
	case ledgerstate.Input:
		err = e.aliasManager.AddInputAlias(t, aliasName)
	}
	return
}

// ClearAliases remove all registered alias names.
func (e *EvilWallet) ClearAliases() {
	e.aliasManager.ClearAliases()
}

// CreateTransaction creates a transaction with the given aliasName and options.
func (e *EvilWallet) CreateTransaction(aliasName string, options ...Option) (tx *ledgerstate.Transaction, err error) {
	buildOptions := NewOptions(options...)

	if len(buildOptions.inputs) == 0 || len(buildOptions.outputs) == 0 {
		return
	}

	inputs := make([]ledgerstate.Input, 0)
	// get inputs by alias
	for inputAlias := range buildOptions.inputs {
		inputs = append(inputs, e.aliasManager.GetInput(inputAlias))
	}

	outputs := make([]ledgerstate.Output, 0)
	addrAliasMap := make(map[ledgerstate.Address]string)
	for alias, balance := range buildOptions.outputs {
		addr := buildOptions.issuer.Address().Address()
		output := ledgerstate.NewSigLockedSingleOutput(balance, addr)
		if err != nil {
			return
		}
		outputs = append(outputs, output)
		addrAliasMap[addr] = alias
	}

	tx, err = e.makeTransaction(ledgerstate.NewInputs(inputs...), ledgerstate.NewOutputs(outputs...), buildOptions.issuer)
	if err != nil {
		return
	}

	for _, output := range tx.Essence().Outputs() {
		// register output alias
		e.aliasManager.AddOutputAlias(output, addrAliasMap[output.Address()])

		// register output as unspent output(input)
		input := ledgerstate.NewUTXOInput(output.ID())
		e.aliasManager.AddInputAlias(input, addrAliasMap[output.Address()])
	}

	e.aliasManager.AddTransactionAlias(tx, aliasName)
	return
}

func (e *EvilWallet) makeTransaction(inputs ledgerstate.Inputs, outputs ledgerstate.Outputs, w *Wallet) (tx *ledgerstate.Transaction, err error) {
	txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, inputs, outputs)
	unlockBlocks := make([]ledgerstate.UnlockBlock, len(txEssence.Inputs()))
	for i, input := range txEssence.Inputs() {
		address, err := e.getAddressFromInput(input)
		if err != nil {
			return nil, err
		}
		unlockBlocks[i] = ledgerstate.NewSignatureUnlockBlock(w.sign(address, txEssence))
	}
	return ledgerstate.NewTransaction(txEssence, unlockBlocks), nil
}

func (e *EvilWallet) getAddressFromInput(input ledgerstate.Input) (addr ledgerstate.Address, err error) {
	typeCastedInput, ok := input.(*ledgerstate.UTXOInput)
	if !ok {
		err = errors.New("wrong type of input")
		return
	}
	addr = e.outputManager.GetOutput(typeCastedInput.ReferencedOutputID()).Address
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region EvilScenario ///////////////////////////////////////////////////////////////////////////////////////////////////////

type EvilScenario struct {
	// todo this should have instructions for evil wallet
	// how to handle this spamming scenario, which input wallet use,
	// where to store outputs of spam ect.
	// All logic of conflict creation will be hidden from spammer or integration test users
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
