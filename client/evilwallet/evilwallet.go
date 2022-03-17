package evilwallet

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/types"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/plugins/faucet"

	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

const (
	GoFConfirmed             = 3
	waitForConfirmation      = 60 * time.Second
	waitForTxSolid           = 2 * time.Second
	FaucetRequestSplitNumber = 100

	maxGoroutines = 5
)

var clientsURL = []string{"http://localhost:8080", "http://localhost:8090"}
var faucetBalance = ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
	ledgerstate.ColorIOTA: uint64(faucet.Parameters.TokensPerRequest),
})

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
func (e *EvilWallet) NewWallet(wType ...WalletType) *Wallet {
	walletType := other
	if len(wType) != 0 {
		walletType = wType[0]
	}
	return e.wallets.NewWallet(walletType)
}

// GetClients returns the given number of clients.
func (e *EvilWallet) GetClients(num int) []*client.GoShimmerAPI {
	return e.connector.GetClients(num)
}

// Connector give access to the EvilWallet connector.
func (e *EvilWallet) Connector() Clients {
	return e.connector
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region EvilWallet Faucet Requests ///////////////////////////////////////////////////////////////////////////////////

// RequestFundsFromFaucet requests funds from the faucet, then track the confirmed status of unspent output,
// also register the alias name for the unspent output if provided.
func (e *EvilWallet) RequestFundsFromFaucet(options ...FaucetRequestOption) (err error) {
	initWallet := e.NewWallet(fresh)
	addr := initWallet.Address()
	buildOptions := NewFaucetRequestOptions(options...)

	addrStr := addr.Base58()

	// request funds from faucet
	err = e.connector.SendFaucetRequest(addrStr)
	if err != nil {
		return
	}
	// track output in output manager and make sure it's confirmed

	out := e.outputManager.CreateOutputFromAddress(initWallet, addr, faucetBalance)
	if out == nil {
		err = errors.New("no outputIDs found on address ")
		return
	}

	allConfirmed := e.outputManager.Track([]ledgerstate.OutputID{out.OutputID})
	if !allConfirmed {
		err = errors.New("output not confirmed")
		return
	}

	if buildOptions.outputAliasName != "" {
		input := ledgerstate.NewUTXOInput(out.OutputID)
		e.aliasManager.AddInputAlias(input, buildOptions.outputAliasName)
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
		// block if full
		semaphore <- true
		go func(reqNum int) {
			defer wg.Done()
			defer func() {
				// release
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
	initWallet := NewWallet()
	funds, err := e.requestAndSplitFaucetFunds(initWallet)
	if err != nil {
		return
	}
	w := e.NewWallet(fresh)
	txIDs := e.splitOutputs(funds, w, FaucetRequestSplitNumber)

	e.outputManager.AwaitTransactionsConfirmation(txIDs, maxGoroutines)
	err = e.outputManager.UpdateOutputsFromTxs(txIDs)
	if err != nil {
		return
	}
	e.wallets.SetWalletReady(w)
	return
}

// RequestFreshFaucetWallet creates a new wallet and fills the wallet with 100 outputs created from funds
// requested from the Faucet.
func (e *EvilWallet) RequestFreshFaucetWallet() (wallet *Wallet, err error) {
	initWallet := NewWallet()
	wallet, err = e.requestAndSplitFaucetFunds(initWallet)
	if err != nil {
		return
	}
	e.wallets.SetWalletReady(wallet)
	return
}

func (e *EvilWallet) requestAndSplitFaucetFunds(initWallet *Wallet) (wallet *Wallet, err error) {
	_, err = e.requestFaucetFunds(initWallet)
	if err != nil {
		return
	}
	//first split 1 to FaucetRequestSplitNumber outputs
	wallet = e.NewWallet(fresh)
	//e.outputManager.AwaitWalletOutputsToBeConfirmed(initWallet)
	txIDs := e.splitOutputs(initWallet, wallet, FaucetRequestSplitNumber)
	e.outputManager.AwaitTransactionsConfirmation(txIDs, maxGoroutines)
	err = e.outputManager.UpdateOutputsFromTxs(txIDs)
	if err != nil {
		return
	}
	return
}

func (e *EvilWallet) requestFaucetFunds(wallet *Wallet) (outputID ledgerstate.OutputID, err error) {
	addr := wallet.Address()
	err = e.connector.SendFaucetRequest(addr.Base58())
	if err != nil {
		return
	}
	output := e.outputManager.CreateOutputFromAddress(wallet, addr, faucetBalance)
	if output == nil {
		err = errors.New("could not get output from a given address")
		return
	}
	ok := e.outputManager.Track([]ledgerstate.OutputID{output.OutputID})
	if !ok {
		err = errors.New("not all outputs has been confirmed")
		return
	}
	outputID = output.OutputID
	return
}

func (e *EvilWallet) splitOutputs(inputWallet *Wallet, outputWallet *Wallet, splitNumber int) []string {
	wg := sync.WaitGroup{}

	txIDs := make([]string, inputWallet.UnspentOutputsLength())
	if inputWallet.IsEmpty() {
		return []string{}
	}
	// Add all aliases before creating txs
	inputAliases, outputAliases, txAliases := e.handleAliasesDuringSplitOutputs(outputWallet, splitNumber, inputWallet)
	inputNum := 0

	for _, input := range inputWallet.UnspentOutputs() {
		wg.Add(1)
		go func(inputNum int, input *Output) {
			defer wg.Done()
			tx, err := e.CreateTransaction(txAliases[inputNum], WithInputs(inputAliases[inputNum]), WithOutputs(outputAliases[inputNum]),
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

func (e *EvilWallet) handleAliasesDuringSplitOutputs(outputWallet *Wallet, splitNumber int, inputWallet *Wallet) ([]string, [][]string, []string) {
	inputAliases, AllOutputsAliases, txAliases := make([]string, 0), make([][]string, 0), make([]string, 0)
	n := inputWallet.UnspentOutputsLength()
	inputAliases = e.aliasManager.CreateAliasesForInputs(n)
	txAliases = e.aliasManager.CreateAliasesForTransactions(n, outputWallet.ID, inputWallet.ID)
	for i := 0; i < n; i++ {
		outputAliases := e.aliasManager.CreateAliasesForOutputs(outputWallet.ID, splitNumber)
		AllOutputsAliases = append(AllOutputsAliases, outputAliases)
	}
	return inputAliases, AllOutputsAliases, txAliases
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region EvilWallet functionality ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// ClearAliases remove all registered alias names.
func (e *EvilWallet) ClearAliases() {
	e.aliasManager.ClearAliases()
}

func (e *EvilWallet) PrepareCustomConflicts(conflictsMaps []ConflictMap, outputWallet *Wallet) (conflictBatch [][]*ledgerstate.Transaction, err error) {
	if outputWallet == nil {
		return nil, errors.Errorf("no output wallet provided")
	}
	for _, conflictMap := range conflictsMaps {
		var txs []*ledgerstate.Transaction
		for txAlias, options := range conflictMap {
			options = append(options, WithOutputWallet(outputWallet))
			tx, err2 := e.CreateTransaction(txAlias, options...)
			if err2 != nil {
				return nil, err2
			}
			txs = append(txs, tx)
		}
		conflictBatch = append(conflictBatch, txs)
	}
	return
}

// SendCustomConflicts sends transactions with the given conflictsMaps.
func (e *EvilWallet) SendCustomConflicts(conflictsMaps []ConflictMap, clients []*client.GoShimmerAPI) (err error) {
	outputWallet := e.NewWallet()
	conflictBatch, err := e.PrepareCustomConflicts(conflictsMaps, outputWallet)
	if err != nil {
		return err
	}
	for _, txs := range conflictBatch {
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

		// wait until transactions are solid
		time.Sleep(waitForTxSolid)
	}
	return
}

// CreateTransaction creates a transaction with the given outputAliasName and options.
func (e *EvilWallet) CreateTransaction(aliasName string, options ...Option) (tx *ledgerstate.Transaction, err error) {
	buildOptions := NewOptions(options...)

	if buildOptions.outputWallet == nil {
		buildOptions.outputWallet = e.NewWallet()
	}

	if len(buildOptions.inputs) == 0 || len(buildOptions.outputs) == 0 {
		return
	}
	inputs, err := e.matchInputsWithAliases(buildOptions)
	if err != nil {
		return nil, err
	}
	outputs, addrAliasMap, err := e.matchOutputsWithAliases(buildOptions)
	if err != nil {
		return nil, err
	}

	alias, remainder, hasRemainder := e.prepareRemainderOutput(buildOptions, outputs)
	if hasRemainder {
		outputs = append(outputs, remainder)
		addrAliasMap[remainder.Address()] = alias
	}

	tx, err = e.makeTransaction(ledgerstate.NewInputs(inputs...), ledgerstate.NewOutputs(outputs...), buildOptions.issuer)
	if err != nil {
		return nil, err
	}

	err = e.updateOutputIDs(tx.ID(), tx.Essence().Outputs(), buildOptions.outputWallet)
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
		e.aliasManager.AddOutputAlias(output, addrAliasMap[output.Address()])

		// register output as unspent output(input)
		input := ledgerstate.NewUTXOInput(output.ID())
		e.aliasManager.AddInputAlias(input, addrAliasMap[output.Address()])
	}
	return
}

// matchInputsWithAliases gets input from the alias manager. if input was not assigned to an alias before,
// it assigns a new fresh faucet output.
func (e *EvilWallet) matchInputsWithAliases(buildOptions *Options) (inputs []ledgerstate.Input, err error) {
	// get inputs by alias
	for inputAlias := range buildOptions.inputs {
		in, ok := e.aliasManager.GetInput(inputAlias)
		if ok {
			err = e.updateIssuerWalletForAlias(buildOptions, in)
			if err != nil {
				return nil, err
			}
		} else {
			// No output found for given alias, use internal fresh output if wallets are non-empty.
			err = e.getIssuerWallet(buildOptions)
			if err != nil {
				return nil, err
			}

			out := e.wallets.GetUnspentOutput(buildOptions.issuer)
			if out == nil {
				return
			}
			in = ledgerstate.NewUTXOInput(out.OutputID)
			e.aliasManager.AddInputAlias(in, inputAlias)
		}
		inputs = append(inputs, in)
	}
	return inputs, nil
}

func (e *EvilWallet) getIssuerWallet(buildOptions *Options) error {
	// if input wallet is not specified, use fresh faucet wallet
	if buildOptions.issuer == nil {
		if wallet, err2 := e.wallets.FreshWallet(); wallet != nil {
			buildOptions.issuer = wallet
		} else {
			return errors.Newf("no fresh wallet is available: %w", err2)
		}
	}
	return nil
}

func (e *EvilWallet) updateIssuerWalletForAlias(buildOptions *Options, in ledgerstate.Input) error {
	inputWallet := e.outputManager.outputIDWalletMap[in.Base58()]
	if buildOptions.issuer == nil {
		buildOptions.issuer = inputWallet
	}
	if buildOptions.issuer.ID != inputWallet.ID {
		return errors.New("provided inputs had to belong to the same wallets")
	}
	return nil
}

// matchOutputsWithAliases creates outputs based on balances provided via options.
// Outputs are not yet added to the Alias Manager, as they have no ID before the transaction is created.
// Thus, they are tracker in address to alias map.
func (e *EvilWallet) matchOutputsWithAliases(buildOptions *Options) (outputs []ledgerstate.Output, addrAliasMap map[ledgerstate.Address]string, err error) {
	err = e.updateOutputBalances(buildOptions)
	if err != nil {
		return nil, nil, err
	}
	addrAliasMap = make(map[ledgerstate.Address]string)
	for alias, balance := range buildOptions.outputs {
		evilOutput := e.outputManager.CreateEmptyOutput(buildOptions.outputWallet, balance)
		output := ledgerstate.NewSigLockedColoredOutput(balance, evilOutput.Address)
		if err != nil {
			return nil, nil, err
		}
		outputs = append(outputs, output)
		addrAliasMap[evilOutput.Address] = alias
	}

	return
}

func (e *EvilWallet) prepareRemainderOutput(buildOptions *Options, outputs []ledgerstate.Output) (alias string, remainderOutput ledgerstate.Output, added bool) {
	inputBalance := uint64(0)
	var remainderAddress ledgerstate.Address
	for inputAlias := range buildOptions.inputs {
		in, _ := e.aliasManager.GetInput(inputAlias)

		// get balance from output manager
		outputID, _ := ledgerstate.OutputIDFromBase58(in.Base58())
		output := e.outputManager.GetOutput(outputID)
		output.Balance.ForEach(func(color ledgerstate.Color, balance uint64) bool {
			inputBalance += balance
			return true
		})
		if alias == "" {
			remainderAddress, _ = e.getAddressFromInput(in)
			alias = inputAlias
		}
	}

	outputBalance := uint64(0)
	for _, o := range outputs {
		o.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
			outputBalance += balance
			return true
		})
	}

	// remainder balances is sent to one of the address in inputs
	if outputBalance < inputBalance {
		remainderOutput = ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: inputBalance - outputBalance,
		}), remainderAddress)
		added = true
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
			// get balance from output manager
			outputID, _ := ledgerstate.OutputIDFromBase58(in.Base58())
			output := e.outputManager.GetOutput(outputID)
			output.Balance.ForEach(func(color ledgerstate.Color, balance uint64) bool {
				totalBalance += balance
				return true
			})
		}
		balances := SplitBalanceEqually(len(buildOptions.outputs), totalBalance)

		i := 0
		for out := range buildOptions.outputs {
			buildOptions.outputs[out] = ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
				ledgerstate.ColorIOTA: balances[i],
			})
			i++
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

func (e *EvilWallet) updateOutputIDs(txID ledgerstate.TransactionID, outputs ledgerstate.Outputs, outWallet *Wallet) error {
	for i, output := range outputs {
		outID := output.SetID(ledgerstate.NewOutputID(txID, uint16(i))).ID()
		err := e.outputManager.UpdateOutputID(outWallet, output.Address().Base58(), outID)
		if err != nil {
			return err
		}
	}
	return nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region EvilScenario ///////////////////////////////////////////////////////////////////////////////////////////////////////

type EvilBatch []ConflictMap

type EvilScenario struct {
	conflictBatch EvilBatch
	repeat        int
	// determines whether outputs of the batch  should be reused during the spam to create deep UTXO tree structure.
	reuse bool

	// outputs of the batch that can be reused in deep spamming by collecting them in reuse wallet.
	batchOutputsAliases map[string]types.Empty
}

func NewEvilScenario(conflictBatch []ConflictMap, repeat int, reuse bool) {
	scenario := &EvilScenario{
		repeat: repeat,
		reuse:  reuse,
	}

	if conflictBatch == nil {
		scenario.conflictBatch = SingleTransactionBatch()
	} else {
		scenario.conflictBatch = conflictBatch
		scenario.assignBatchOutputs()
	}
}

func (e *EvilScenario) assignBatchOutputs() {
	e.batchOutputsAliases = make(map[string]types.Empty)
	for _, conflictMap := range e.conflictBatch {
		for _, options := range conflictMap {
			option := NewOptions(options...)
			for outputAlis := range option.outputs {
				// add output aliases that are not used in this conflict batch
				if _, ok := option.inputs[outputAlis]; !ok {
					e.batchOutputsAliases[outputAlis] = types.Void
				}
			}
		}
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
