package evilwallet

import (
	"errors"
	"fmt"
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
	wallets := NewWallets()
	return &EvilWallet{
		wallets:         wallets,
		connector:       connector,
		outputManager:   NewOutputManager(connector, wallets),
		conflictManager: NewConflictManager(),
		aliasManager:    NewAliasManager(),
	}
}

// NewWallet creates a new wallet of the given wallet type.
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
func (e *EvilWallet) RequestFundsFromFaucet(wallet *Wallet, options ...FaucetRequestOption) (err error) {
	addr := wallet.Address()
	buildOptions := NewFaucetRequestOptions(options...)

	addrStr := addr.Base58()

	// request funds from faucet
	err = e.connector.SendFaucetRequest(addrStr)
	if err != nil {
		return
	}
	// track output in output manager and make sure it's confirmed
	out := e.outputManager.CreateOutputFromAddress(wallet, addr, uint64(faucet.Parameters.TokensPerRequest))
	if out == nil {
		err = errors.New("no outputIDs found on address ")
		return
	}

	allConfirmed := e.outputManager.Track([]ledgerstate.OutputID{out.OutputID})
	if !allConfirmed {
		err = errors.New("output not confirmed")
		return
	}

	if len(buildOptions.aliasName) > 0 {
		input := ledgerstate.NewUTXOInput(out.OutputID)
		err = e.aliasManager.AddInputAlias(input, "1")
	}

	return
}

// RequestFreshBigFaucetWallets creates n new wallets, each wallet is created from one faucet request and contains 10000 outputs.
func (e *EvilWallet) RequestFreshBigFaucetWallets(numberOfWallets int) {
	// channel to block the number of concurrent goroutines
	semaphore := make(chan bool, maxGoroutines)
	wg := sync.WaitGroup{}

	for reqNum := 0; reqNum < numberOfWallets; reqNum++ {
		wg.Add(1)
		go func(reqNum int) {
			defer wg.Done()
			// block and release goroutines
			semaphore <- true
			defer func() {
				<-semaphore
			}()

			err := e.RequestFreshBigFaucetWallet()
			if err != nil {
				return
			}
		}(reqNum)
	}
	wg.Wait()
	return
}

// RequestFreshBigFaucetWallet creates a new wallet and fills the wallet with 10000 outputs created from funds
// requested from the Faucet.
func (e *EvilWallet) RequestFreshBigFaucetWallet() (err error) {
	initWallet := NewWallet(fresh)
	funds, err := e.requestAndSplitFaucetFunds(initWallet)
	if err != nil {
		return
	}
	w := e.NewWallet(fresh)
	txIDs := e.splitOutputs(funds, w, FaucetRequestSplitNumber)

	e.outputManager.AwaitTransactionsConfirmation(txIDs, maxGoroutines)
	err = e.outputManager.UpdateOutputsFromTxs(w, txIDs)
	if err != nil {
		return
	}
	return
}

// RequestFreshFaucetWallet creates a new wallet and fills the wallet with 100 outputs created from funds
// requested from the Faucet.
func (e *EvilWallet) RequestFreshFaucetWallet() (wallet *Wallet, err error) {
	initWallet := e.NewWallet(fresh)
	wallet, err = e.requestAndSplitFaucetFunds(initWallet)
	if err != nil {
		return
	}

	return
}

func (e *EvilWallet) requestAndSplitFaucetFunds(initWallet *Wallet) (wallet *Wallet, err error) {
	_ = e.requestFaucetFunds(initWallet)

	//first split 1 to FaucetRequestSplitNumber outputs
	wallet = NewWallet(fresh)
	//e.outputManager.AwaitWalletOutputsToBeConfirmed(initWallet)
	txIDs := e.splitOutputs(initWallet, wallet, FaucetRequestSplitNumber)
	e.outputManager.AwaitTransactionsConfirmation(txIDs, maxGoroutines)
	err = e.outputManager.UpdateOutputsFromTxs(wallet, txIDs)
	if err != nil {
		return
	}
	return
}

func (e *EvilWallet) requestFaucetFunds(wallet *Wallet) (outputID ledgerstate.OutputID) {
	addr := wallet.Address()
	err := e.connector.SendFaucetRequest(addr.Base58())
	if err != nil {
		return
	}
	output := e.outputManager.CreateOutputFromAddress(wallet, addr, uint64(faucet.Parameters.TokensPerRequest))
	ok := e.outputManager.Track([]ledgerstate.OutputID{output.OutputID})
	if !ok {
		return
	}
	outputID = output.OutputID
	return
}

func (e *EvilWallet) splitOutputs(inputWallet *Wallet, outputWallet *Wallet, splitNumber int) []string {
	wg := sync.WaitGroup{}

	txIDs := make([]string, len(inputWallet.unspentOutputs))
	if inputWallet.unspentOutputs == nil {
		return []string{}
	}
	// Add all aliases before creating txs
	allInputAliases, allOutputAliases, txAliases := e.handleAliasesDuringSplitOutputs(outputWallet, splitNumber, inputWallet)
	inputNum := 0

	for _, input := range inputWallet.unspentOutputs {
		wg.Add(1)
		go func(inputNum int, input *Output) {
			defer wg.Done()
			tx, err := e.CreateTransaction(txAliases[inputNum], WithInputs(allInputAliases[inputNum]...), WithOutputs(allOutputAliases[inputNum]),
				WithIssuer(inputWallet), WithOutputWallet(outputWallet))

			clt := e.connector.GetClient()
			txID, err := e.connector.PostTransaction(tx, clt)
			if err != nil {
				return
			}
			txIDs[inputNum] = txID.Base58()
		}(inputNum, input)
		inputNum++
	}
	wg.Wait()
	return txIDs
}

func (e *EvilWallet) handleAliasesDuringSplitOutputs(outputWallet *Wallet, splitNumber int, inputWallet *Wallet) ([][]string, [][]string, []string) {
	allInputAliases, allOutputAliases, txAliases := make([][]string, 0), make([][]string, 0), make([]string, 0)
	for _, input := range inputWallet.unspentOutputs {
		inputs := []*Output{input}

		inputAliases := e.aliasManager.CreateAliasesForInputs(len(inputs))
		e.aliasManager.AddInputAliases(inputs, inputAliases)
		outputAliases := e.aliasManager.CreateAliasesForOutputs(outputWallet.ID, splitNumber)
		txAlias := e.aliasManager.CreateAliasForTransaction(outputWallet.ID, inputWallet.ID, input.OutputID.Base58())

		allInputAliases = append(allInputAliases, inputAliases)
		allOutputAliases = append(allOutputAliases, outputAliases)
		txAliases = append(txAliases, txAlias)
	}

	return allInputAliases, allOutputAliases, txAliases
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

// SendCustomConflicts sends transactions with the given conflictsMaps.
func (e *EvilWallet) SendCustomConflicts(conflictsMaps []ConflictMap, clients []*client.GoShimmerAPI) (err error) {
	for _, conflictMap := range conflictsMaps {
		var txs []*ledgerstate.Transaction
		for txAlias, options := range conflictMap {
			tx, err := e.CreateTransaction(txAlias, options...)
			if err != nil {
				return err
			}
			txs = append(txs, tx)
		}

		if len(txs) > len(clients) {
			return errors.New("insufficient clients to send double spend")
		}

		// send transactions in parallel
		wg := sync.WaitGroup{}
		for i, tx := range txs {
			wg.Add(1)
			go func(clt *client.GoShimmerAPI, tx *ledgerstate.Transaction) {
				defer wg.Done()
				_, _ = clt.PostTransaction(tx.Bytes())
			}(clients[i], tx)
		}
		wg.Wait()
	}
	return
}

// CreateTransaction creates a transaction with the given aliasName and options.
func (e *EvilWallet) CreateTransaction(aliasName string, options ...Option) (tx *ledgerstate.Transaction, err error) {
	buildOptions := NewOptions(options...)

	if len(buildOptions.inputs) == 0 || len(buildOptions.outputs) == 0 {
		return
	}

	inputs := e.prepareInputs(buildOptions)
	outputs, addrAliasMap, err := e.prepareOutputs(buildOptions)

	if err != nil {
		return nil, err
	}

	for _, out := range outputs {
		e.outputManager.CreateEmptyOutput(buildOptions.issuer, buildOptions.outputs[out.ID().Base58()])
	}
	tx, err = e.makeTransaction(ledgerstate.NewInputs(inputs...), ledgerstate.NewOutputs(outputs...), buildOptions.issuer)
	if err != nil {
		return nil, err
	}

	err = e.registerOutputAliases(tx.Essence().Outputs(), addrAliasMap)
	if err != nil {
		return nil, err
	}

	err = e.aliasManager.AddTransactionAlias(tx, aliasName)
	if err != nil {
		return nil, err
	}
	return
}

func (e *EvilWallet) registerOutputAliases(outputs ledgerstate.Outputs, addrAliasMap map[ledgerstate.Address]string) (err error) {
	for _, output := range outputs {
		// register output alias
		err = e.aliasManager.AddOutputAlias(output, addrAliasMap[output.Address()])
		if err != nil {
			return
		}

		// register output as unspent output(input)
		input := ledgerstate.NewUTXOInput(output.ID())
		err = e.aliasManager.AddInputAlias(input, addrAliasMap[output.Address()])
		if err != nil {
			return
		}
	}
	return
}

func (e *EvilWallet) prepareInputs(buildOptions *Options) (inputs []ledgerstate.Input) {
	// get inputs by alias
	for inputAlias := range buildOptions.inputs {
		in, ok := e.aliasManager.GetInput(inputAlias)
		// No output found for given alias, use internal fresh output if wallets are non-empty.
		if !ok {
			out := e.wallets.GetUnspentOutput(fresh)
			if out == nil {
				return
			}
			in = ledgerstate.NewUTXOInput(out.OutputID)
		}
		inputs = append(inputs, in)
	}
	return inputs
}

func (e *EvilWallet) prepareOutputs(buildOptions *Options) (outputs []ledgerstate.Output, addrAliasMap map[ledgerstate.Address]string, err error) {
	err = e.updateOutputBalances(buildOptions)
	if err != nil {
		return nil, nil, err
	}
	addrAliasMap = make(map[ledgerstate.Address]string)
	for alias, balance := range buildOptions.outputs {
		evilOutput := e.outputManager.CreateEmptyOutput(buildOptions.outputWallet, balance)
		output := ledgerstate.NewSigLockedSingleOutput(balance, evilOutput.Address)
		if err != nil {
			return nil, nil, err
		}
		outputs = append(outputs, output)
		addrAliasMap[evilOutput.Address] = alias
	}
	return
}

func (e *EvilWallet) updateOutputBalances(buildOptions *Options) (err error) {
	if !buildOptions.isBalanceProvided() {
		totalBalance := uint64(0)
		for inputAlias := range buildOptions.inputs {
			in, ok := e.aliasManager.GetInput(inputAlias)
			if !ok {
				err = errors.New("could not get input by input alias")
				return
			}
			addr, err2 := e.getAddressFromInput(in)
			if err2 != nil {
				err = err2
				return
			}
			bal := buildOptions.issuer.UnspentOutputBalance(addr.Base58())
			totalBalance += bal
		}
		balances := SplitBalanceEqually(len(buildOptions.outputs), totalBalance)
		i := 0
		for out := range buildOptions.outputs {
			buildOptions.outputs[out] = balances[i]
			i += 1
		}
	}
	return
}

func (e *EvilWallet) makeTransaction(inputs ledgerstate.Inputs, outputs ledgerstate.Outputs, w *Wallet) (tx *ledgerstate.Transaction, err error) {
	txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, inputs, outputs)
	unlockBlocks := make([]ledgerstate.UnlockBlock, len(txEssence.Inputs()))
	for i, input := range txEssence.Inputs() {
		addr, err2 := e.getAddressFromInput(input)
		if err2 != nil {
			return nil, err2
		}
		unlockBlocks[i] = ledgerstate.NewSignatureUnlockBlock(w.sign(addr, txEssence))
	}
	return ledgerstate.NewTransaction(txEssence, unlockBlocks), nil
}

func (e *EvilWallet) getAddressFromInput(input ledgerstate.Input) (addr ledgerstate.Address, err error) {
	typeCastedInput, ok := input.(*ledgerstate.UTXOInput)
	if !ok {
		err = errors.New("wrong type of input")
		return
	}

	refOut := typeCastedInput.ReferencedOutputID()
	out := e.outputManager.GetOutput(refOut)
	if out == nil {
		err = errors.New("output not found in output manager")
		return
	}
	addr = out.Address
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
