package evilwallet

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/client/wallet/packages/address"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/plugins/faucet"

	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

const (
	// GoFConfirmed defines the grade of finality that is considered confirmed.
	GoFConfirmed = 3
	// FaucetRequestSplitNumber defines the number of outputs to split from a faucet request.
	FaucetRequestSplitNumber = 100

	waitForConfirmation   = 60 * time.Second
	waitForSolidification = 10 * time.Second

	awaitConfirmationSleep   = time.Second
	awaitSolidificationSleep = time.Millisecond * 500

	WaitForTxSolid = 2 * time.Second

	maxGoroutines = 5
)

var defaultClientsURLs = []string{"http://localhost:8080", "http://localhost:8090"}
var faucetBalance = ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
	ledgerstate.ColorIOTA: uint64(faucet.Parameters.TokensPerRequest),
})

// region EvilWallet ///////////////////////////////////////////////////////////////////////////////////////////////////////

// EvilWallet provides a user-friendly way to do complicated double spend scenarios.
type EvilWallet struct {
	wallets         *Wallets
	connector       Connector
	outputManager   *OutputManager
	conflictManager *ConflictManager
	aliasManager    *AliasManager
}

// NewEvilWallet creates an EvilWallet instance.
func NewEvilWallet(clientsUrls ...string) *EvilWallet {
	urls := clientsUrls
	if len(urls) == 0 {
		urls = append(urls, defaultClientsURLs...)
	}

	connector := NewWebClients(urls)
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
	walletType := Other
	if len(wType) != 0 {
		walletType = wType[0]
	}
	return e.wallets.NewWallet(walletType)
}

// GetClients returns the given number of clients.
func (e *EvilWallet) GetClients(num int) []Client {
	return e.connector.GetClients(num)
}

// Connector give access to the EvilWallet connector.
func (e *EvilWallet) Connector() Connector {
	return e.connector
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region EvilWallet Faucet Requests ///////////////////////////////////////////////////////////////////////////////////

// RequestFundsFromFaucet requests funds from the faucet, then track the confirmed status of unspent output,
// also register the alias name for the unspent output if provided.
func (e *EvilWallet) RequestFundsFromFaucet(options ...FaucetRequestOption) (err error, initWallet *Wallet) {
	initWallet = e.NewWallet(Fresh)
	addr := initWallet.Address()
	buildOptions := NewFaucetRequestOptions(options...)

	addrStr := addr.Base58()

	// request funds from faucet
	clt := e.connector.GetClient()
	err = clt.SendFaucetRequest(addrStr)
	if err != nil {
		return
	}

	out := e.outputManager.CreateOutputFromAddress(initWallet, addr, faucetBalance)
	if out == nil {
		err = errors.New("no outputIDs found on address ")
		return
	}

	// track output in output manager and make sure it's confirmed
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
		go func() {
			defer wg.Done()
			defer func() {
				// release
				<-semaphore
			}()

			err := e.RequestFreshBigFaucetWallet()
			if err != nil {
				return
			}
		}()
	}
	wg.Wait()
}

// RequestFreshBigFaucetWallet creates a new wallet and fills the wallet with 10000 outputs created from funds
// requested from the Faucet.
func (e *EvilWallet) RequestFreshBigFaucetWallet() (err error) {
	initWallet := NewWallet()
	funds, err := e.requestAndSplitFaucetFunds(initWallet)
	if err != nil {
		return
	}
	w := e.NewWallet(Fresh)
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
func (e *EvilWallet) RequestFreshFaucetWallet() (err error) {
	initWallet := NewWallet()
	wallet, err := e.requestAndSplitFaucetFunds(initWallet)
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
	wallet = e.NewWallet(Fresh)
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
	clt := e.connector.GetClient()
	err = clt.SendFaucetRequest(addr.Base58())
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

func (e *EvilWallet) splitOutputs(inputWallet, outputWallet *Wallet, splitNumber int) []string {
	wg := sync.WaitGroup{}

	txIDs := make([]string, inputWallet.UnspentOutputsLength())
	if inputWallet.IsEmpty() {
		return []string{}
	}

	inputNum := 0
	for addr := range inputWallet.UnspentOutputs() {
		wg.Add(1)
		go func(inputNum int, addr string) {
			defer wg.Done()

			input, outputs := e.handleInputOutputDuringSplitOutputs(splitNumber, inputWallet, addr)

			tx, err := e.CreateTransaction(WithInputs(input), WithOutputs(outputs),
				WithIssuer(inputWallet), WithOutputWallet(outputWallet))
			if err != nil {
				return
			}

			clt := e.connector.GetClient()
			txID, err := clt.PostTransaction(tx)
			if err != nil {
				return
			}
			txIDs[inputNum] = txID.Base58()
		}(inputNum, addr)
		inputNum++
	}
	wg.Wait()
	return txIDs
}

func (e *EvilWallet) handleInputOutputDuringSplitOutputs(splitNumber int, inputWallet *Wallet, inputAddr string) (input ledgerstate.OutputID, outputs []*OutputOption) {
	evilInput := inputWallet.UnspentOutput(inputAddr)
	input = evilInput.OutputID

	inputBalance := uint64(0)
	evilInput.Balance.ForEach(func(color ledgerstate.Color, balance uint64) bool {
		inputBalance += balance
		return true
	})

	balances := SplitBalanceEqually(splitNumber, inputBalance)
	for _, bal := range balances {
		outputs = append(outputs, &OutputOption{amount: bal})
	}
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region EvilWallet functionality ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// ClearAliases remove only provided aliases from AliasManager.
func (e *EvilWallet) ClearAliases(aliases ScenarioAlias) {
	e.aliasManager.ClearAliases(aliases)
}

// ClearAllAliases remove all registered alias names.
func (e *EvilWallet) ClearAllAliases() {
	e.aliasManager.ClearAllAliases()
}

func (e *EvilWallet) PrepareCustomConflicts(conflictsMaps []ConflictSlice) (conflictBatch [][]*ledgerstate.Transaction, err error) {
	for _, conflictMap := range conflictsMaps {
		var txs []*ledgerstate.Transaction
		for _, options := range conflictMap {
			tx, err2 := e.CreateTransaction(options...)
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
func (e *EvilWallet) SendCustomConflicts(conflictsMaps []ConflictSlice) (err error) {
	conflictBatch, err := e.PrepareCustomConflicts(conflictsMaps)
	if err != nil {
		return err
	}
	for _, txs := range conflictBatch {
		clients := e.connector.GetClients(len(txs))
		if len(txs) > len(clients) {
			return errors.New("insufficient clients to send conflicts")
		}

		// send transactions in parallel
		wg := sync.WaitGroup{}
		for i, tx := range txs {
			wg.Add(1)
			go func(clt Client, tx *ledgerstate.Transaction) {
				defer wg.Done()
				_, _ = clt.PostTransaction(tx)
			}(clients[i], tx)
		}
		wg.Wait()

		// wait until transactions are solid
		time.Sleep(WaitForTxSolid)
	}
	return
}

// CreateTransaction creates a transaction based on provided options. If no input wallet is provided, the next non-empty faucet wallet is used.
// Inputs of the transaction are determined in three ways:
// 1 - inputs are provided directly without associated alias, 2- alias is provided, and input is already stored in an alias manager,
// 3 - alias is provided, and there are no inputs assigned in Alias manager, so aliases are assigned to next ready inputs from input wallet.
func (e *EvilWallet) CreateTransaction(options ...Option) (tx *ledgerstate.Transaction, err error) {
	buildOptions := NewOptions(options...)
	// wallet used only for outputs in the middle of the batch, that will never be reused outside custom conflict batch creation.
	tempWallet := e.NewWallet()

	err = buildOptions.checkInputsAndOutputs()
	if err != nil {
		return
	}

	err = e.isWalletProvidedForInputsOutputs(buildOptions)
	if err != nil {
		return nil, err
	}

	err = e.updateInputWallet(buildOptions)
	if err != nil {
		return nil, err
	}

	e.createNewOutputWalletIfNotProvided(buildOptions)

	inputs, err := e.prepareInputs(buildOptions)
	if err != nil {
		return nil, err
	}
	outputs, addrAliasMap, tempAddresses, err := e.prepareOutputs(buildOptions, tempWallet)
	if err != nil {
		return nil, err
	}

	alias, remainder, hasRemainder := e.prepareRemainderOutput(buildOptions, outputs)
	if hasRemainder {
		outputs = append(outputs, remainder)
		if alias != "" && addrAliasMap != nil {
			addrAliasMap[remainder.Address()] = alias
		}
	}

	tx, err = e.makeTransaction(ledgerstate.NewInputs(inputs...), ledgerstate.NewOutputs(outputs...), buildOptions.inputWallet)
	if err != nil {
		return nil, err
	}

	e.updateOutputManager(tx, tempAddresses, buildOptions, tempWallet)

	err = e.updateOutputIDs(tx.Essence().Outputs(), buildOptions.outputWallet, tempWallet, tempAddresses)
	if err != nil {
		return nil, err
	}

	e.registerOutputAliases(tx.Essence().Outputs(), addrAliasMap)

	return
}

// updateOutputManager adds output to the OutputManager if
func (e *EvilWallet) updateOutputManager(tx *ledgerstate.Transaction, tempAddresses map[ledgerstate.Address]types.Empty, buildOptions *Options, tempWallet *Wallet) {
	for _, o := range tx.Essence().Outputs() {
		if e.outputManager.GetOutput(o.ID()) == nil {
			if _, ok := tempAddresses[o.Address()]; ok {
				e.outputManager.AddOutput(buildOptions.outputWallet, o)
			} else {
				e.outputManager.AddOutput(tempWallet, o)
			}
		}
	}
}

// updateInputWallet if input wallet is not specified, or aliases were provided without inputs (batch inputs) use Fresh faucet wallet.
func (e *EvilWallet) updateInputWallet(buildOptions *Options) error {
	for alias := range buildOptions.aliasInputs {
		// inputs provided for aliases (middle inputs in a batch)
		_, ok := e.aliasManager.GetInput(alias)
		if ok {
			// leave nil, wallet will be selected based on OutputIDWalletMap
			buildOptions.inputWallet = nil
			return nil
		}
		break
	}
	err := e.useFreshIfInputWalletNotProvided(buildOptions)
	if err != nil {
		return err
	}
	return nil
}

// isWalletProvidedForInputs checks if inputs without corresponding aliases are provided with corresponding input wallet.
func (e *EvilWallet) isWalletProvidedForInputsOutputs(buildOptions *Options) error {
	if buildOptions.areInputsProvidedWithoutAliases() {
		if buildOptions.inputWallet == nil {
			return errors.New("no input wallet provided for inputs without aliases")
		}
	}
	if buildOptions.areOutputsProvidedWithoutAliases() {
		if buildOptions.outputWallet == nil {
			return errors.New("no output wallet provided for outputs without aliases")
		}
	}
	return nil
}

func (e *EvilWallet) createNewOutputWalletIfNotProvided(buildOptions *Options) {
	if buildOptions.outputWallet == nil {
		buildOptions.outputWallet = NewWallet()
	}
}

func (e *EvilWallet) registerOutputAliases(outputs ledgerstate.Outputs, addrAliasMap map[ledgerstate.Address]string) {
	if len(addrAliasMap) == 0 {
		return
	}

	for _, output := range outputs {
		// register output alias
		e.aliasManager.AddOutputAlias(output, addrAliasMap[output.Address()])

		// register output as unspent output(input)
		input := ledgerstate.NewUTXOInput(output.ID())
		e.aliasManager.AddInputAlias(input, addrAliasMap[output.Address()])
	}
	return
}

func (e *EvilWallet) prepareInputs(buildOptions *Options) (inputs []ledgerstate.Input, err error) {
	if buildOptions.areInputsProvidedWithoutAliases() {
		for _, out := range buildOptions.inputs {
			inputs = append(inputs, ledgerstate.NewUTXOInput(out))
		}
		return
	}
	// append inputs with alias
	aliasInputs, err := e.matchInputsWithAliases(buildOptions)
	if err != nil {
		return nil, err
	}
	inputs = append(inputs, aliasInputs...)

	return inputs, nil
}

// prepareOutputs creates outputs for different scenarios, if no aliases were provided, new empty outputs are created from buildOptions.outputs balances.
func (e *EvilWallet) prepareOutputs(buildOptions *Options, tempWallet *Wallet) (outputs []ledgerstate.Output,
	addrAliasMap map[ledgerstate.Address]string, tempAddresses map[ledgerstate.Address]types.Empty, err error) {
	// if outputs were provided with aliases
	if !buildOptions.areOutputsProvidedWithoutAliases() {
		outputs, addrAliasMap, tempAddresses, err = e.matchOutputsWithAliases(buildOptions, tempWallet)
	} else {
		for _, balance := range buildOptions.outputs {
			evilOutput := e.outputManager.CreateEmptyOutput(buildOptions.outputWallet, balance)
			output := ledgerstate.NewSigLockedColoredOutput(balance, evilOutput.Address)
			if err != nil {
				return nil, nil, nil, err
			}
			outputs = append(outputs, output)
		}
	}
	return
}

// matchInputsWithAliases gets input from the alias manager. if input was not assigned to an alias before,
// it assigns a new Fresh faucet output.
func (e *EvilWallet) matchInputsWithAliases(buildOptions *Options) (inputs []ledgerstate.Input, err error) {
	// get inputs by alias
	for inputAlias := range buildOptions.aliasInputs {
		in, ok := e.aliasManager.GetInput(inputAlias)
		if !ok {
			// No output found for given alias, use internal Fresh output if wallets are non-empty.
			out := e.wallets.GetUnspentOutput(buildOptions.inputWallet)
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

func (e *EvilWallet) useFreshIfInputWalletNotProvided(buildOptions *Options) error {
	// if input wallet is not specified, use Fresh faucet wallet
	if buildOptions.inputWallet == nil {
		// deep spam enabled and no input reuse wallet provided, use evil wallet reuse wallet if enough outputs are available
		if buildOptions.reuse {
			outputsNeeded := len(buildOptions.inputs)
			if wallet := e.wallets.reuseWallet(outputsNeeded); wallet != nil {
				buildOptions.inputWallet = wallet
				return nil
			}
		}
		if wallet, err := e.wallets.freshWallet(); wallet != nil {
			buildOptions.inputWallet = wallet
		} else {
			return errors.Newf("no Fresh wallet is available: %w", err)
		}
	}
	return nil
}

// matchOutputsWithAliases creates outputs based on balances provided via options.
// Outputs are not yet added to the Alias Manager, as they have no ID before the transaction is created.
// Thus, they are tracker in address to alias map. If the scenario is used, the outputBatchAliases map is provided
// that indicates which outputs should be saved to the outputWallet.All other outputs are created with temporary wallet,
// and their addresses are stored in tempAddresses.
func (e *EvilWallet) matchOutputsWithAliases(buildOptions *Options, tempWallet *Wallet) (outputs []ledgerstate.Output,
	addrAliasMap map[ledgerstate.Address]string, tempAddresses map[ledgerstate.Address]types.Empty, err error) {
	tempAddresses = make(map[ledgerstate.Address]types.Empty)
	err = e.updateOutputBalances(buildOptions)
	if err != nil {
		return nil, nil, nil, err
	}
	addrAliasMap = make(map[ledgerstate.Address]string)
	for alias, balance := range buildOptions.aliasOutputs {
		// only outputs in buildOptions.outputBatchAliases are created with outWallet, for others we use temporary wallet
		var evilOutput *Output
		var output ledgerstate.Output
		if _, ok := buildOptions.outputBatchAliases[alias]; ok {
			evilOutput = e.outputManager.CreateEmptyOutput(buildOptions.outputWallet, balance)
			output = ledgerstate.NewSigLockedColoredOutput(balance, evilOutput.Address)
		} else {
			evilOutput = e.outputManager.CreateEmptyOutput(tempWallet, balance)
			output = ledgerstate.NewSigLockedColoredOutput(balance, evilOutput.Address)
			tempAddresses[output.Address()] = types.Void

		}

		outputs = append(outputs, output)
		addrAliasMap[evilOutput.Address] = alias
	}

	return
}

func (e *EvilWallet) prepareRemainderOutput(buildOptions *Options, outputs []ledgerstate.Output) (alias string, remainderOutput ledgerstate.Output, added bool) {
	inputBalance := uint64(0)

	var remainderAddress ledgerstate.Address
	for inputAlias := range buildOptions.aliasInputs {
		in, _ := e.aliasManager.GetInput(inputAlias)
		// get balance from output manager
		out, _ := ledgerstate.OutputIDFromBase58(in.Base58())
		output := e.outputManager.GetOutput(out)

		output.Balance.ForEach(func(color ledgerstate.Color, balance uint64) bool {
			inputBalance += balance
			return true
		})
		if alias == "" {
			remainderAddress, _ = e.getAddressFromInput(in)
			alias = inputAlias
		}
	}

	for _, input := range buildOptions.inputs {
		// get balance from output manager
		inputDetails := e.outputManager.GetOutput(input)
		inputDetails.Balance.ForEach(func(color ledgerstate.Color, balance uint64) bool {
			inputBalance += balance
			return true
		})
		if remainderAddress == address.AddressEmpty.Address() {
			remainderAddress = inputDetails.Address
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
	// when aliases are not used for outputs, the balance had to be provided in options, nothing to do
	if buildOptions.areOutputsProvidedWithoutAliases() {
		return
	}
	totalBalance := uint64(0)
	if !buildOptions.isBalanceProvided() {

		if buildOptions.areInputsProvidedWithoutAliases() {
			for _, input := range buildOptions.inputs {
				// get balance from output manager
				inputDetails := e.outputManager.GetOutput(input)
				inputDetails.Balance.ForEach(func(color ledgerstate.Color, balance uint64) bool {
					totalBalance += balance
					return true
				})
			}
		} else {
			for inputAlias := range buildOptions.aliasInputs {
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

		}
		balances := SplitBalanceEqually(len(buildOptions.outputs)+len(buildOptions.aliasOutputs), totalBalance)
		i := 0
		for out := range buildOptions.aliasOutputs {
			buildOptions.aliasOutputs[out] = ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
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
		var wallet *Wallet
		if w == nil { // aliases provided with inputs, use wallet saved in outputManager
			wallet = e.outputManager.OutputIDWalletMap(input.Base58())
		} else {
			wallet = w
		}
		unlockBlocks[i] = ledgerstate.NewSignatureUnlockBlock(wallet.Sign(addr, txEssence))
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

func (e *EvilWallet) updateOutputIDs(outputs ledgerstate.Outputs, outWallet *Wallet, tempWallet *Wallet, tempAddresses map[ledgerstate.Address]types.Empty) error {
	for _, output := range outputs {
		var wallet *Wallet
		if _, ok := tempAddresses[output.Address()]; ok {
			wallet = tempWallet
		} else {
			wallet = outWallet
		}
		err := e.outputManager.UpdateOutputID(wallet, output.Address().Base58(), output.ID())
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *EvilWallet) PrepareCustomConflictsSpam(scenario *EvilScenario) (txs [][]*ledgerstate.Transaction, allAliases ScenarioAlias, err error) {
	conflicts, allAliases, err := e.prepareConflictSliceForScenario(scenario)
	if err != nil {
		return
	}
	txs, err = e.PrepareCustomConflicts(conflicts)

	return
}

func (e *EvilWallet) prepareConflictSliceForScenario(scenario *EvilScenario) (conflictSlice []ConflictSlice, allAliases ScenarioAlias, err error) {
	genOutputOptions := func(aliases []string) []*OutputOption {
		outputOptions := make([]*OutputOption, 0)
		for _, o := range aliases {
			outputOptions = append(outputOptions, &OutputOption{aliasName: o})
		}
		return outputOptions
	}

	// make conflictSlice
	prefixedBatch, allAliases, batchOutputs := scenario.ConflictBatchWithPrefix()
	conflictSlice = make([]ConflictSlice, 0)
	for _, conflictMap := range prefixedBatch {
		conflicts := make([][]Option, 0)
		for _, aliases := range conflictMap {
			outs := genOutputOptions(aliases.Outputs)
			option := []Option{WithInputs(aliases.Inputs), WithOutputs(outs)}
			option = append(option, WithOutputBatchAliases(batchOutputs))
			if scenario.OutputWallet != nil {
				option = append(option, WithOutputWallet(scenario.OutputWallet))
			}
			if scenario.RestrictedInputWallet != nil {
				option = append(option, WithIssuer(scenario.RestrictedInputWallet))
			}
			if scenario.Reuse {
				option = append(option, WithReuseOutputs())
			}
			conflicts = append(conflicts, option)
		}
		conflictSlice = append(conflictSlice, conflicts)
	}

	return
}

func (e *EvilWallet) AwaitInputsSolidity(inputs ledgerstate.Inputs, clt Client) {
	awaitSolid := make([]string, 0)
	for _, in := range inputs {
		awaitSolid = append(awaitSolid, in.Base58())
	}
	e.outputManager.AwaitOutputsToBeSolid(awaitSolid, clt, maxGoroutines)
}

func (e *EvilWallet) SetTxOutputsSolid(outputs ledgerstate.Outputs, clientID string) {
	for _, out := range outputs {
		e.outputManager.SetOutputIDSolidForIssuer(out.ID().Base58(), clientID)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region EvilScenario ///////////////////////////////////////////////////////////////////////////////////////////////////////

// The custom conflict in spammer can be provided like this:
// EvilBatch{
// 	{
// 		ScenarioAlias{inputs: []{"1"}, outputs: []{"2","3"}}
// 	},
// 	{
// 		ScenarioAlias{inputs: []{"2"}, outputs: []{"4"}},
// 		ScenarioAlias{inputs: []{"2"}, outputs: []{"5"}}
// 	}
// }

type ScenarioAlias struct {
	Inputs  []string
	Outputs []string
}

func NewScenarioAlias() ScenarioAlias {
	return ScenarioAlias{
		Inputs:  make([]string, 0),
		Outputs: make([]string, 0),
	}
}

type EvilBatch [][]ScenarioAlias

type EvilScenario struct {
	ID string
	// provides a user-friendly way of listing input and output aliases
	ConflictBatch EvilBatch
	// determines whether outputs of the batch  should be reused during the spam to create deep UTXO tree structure.
	Reuse bool
	// if provided, the outputs from the spam will be saved into this wallet, accepted types of wallet: Reuse, RestrictedReuse.
	// if type == Reuse, then wallet is available for reuse spamming scenarios that did not provide RestrictedWallet.
	OutputWallet *Wallet
	// if provided and reuse set to true, outputs from this wallet will be used for deep spamming, allows for controllable building of UTXO deep structures.
	// if not provided evil wallet will use Reuse wallet if any is available. Accepts only RestrictedReuse wallet type.
	RestrictedInputWallet *Wallet
	// used together with scenario ID to create a prefix for distinct batch alias creation
	BatchesCreated *atomic.Uint64
}

func NewEvilScenario(options ...ScenarioOption) *EvilScenario {
	scenario := &EvilScenario{
		ConflictBatch:  SingleTransactionBatch(),
		Reuse:          false,
		OutputWallet:   NewWallet(),
		BatchesCreated: atomic.NewUint64(0),
	}

	for _, option := range options {
		option(scenario)
	}
	scenario.ID = base58.Encode([]byte(fmt.Sprintf("%v%v%v", scenario.ConflictBatch, scenario.Reuse, scenario.OutputWallet.ID)))[:11]
	return scenario
}

// readCustomConflictsPattern determines outputs of the batch, needed for saving batch outputs to the outputWallet.
func (e *EvilScenario) readCustomConflictsPattern(batch EvilBatch) (batchOutputs map[string]types.Empty) {
	outputs := make(map[string]types.Empty)
	inputs := make(map[string]types.Empty)

	for _, conflictMap := range batch {
		for _, conflicts := range conflictMap {
			// add output to outputsAliases
			for _, input := range conflicts.Inputs {
				inputs[input] = types.Void
			}
			for _, output := range conflicts.Outputs {
				outputs[output] = types.Void
			}
		}
	}
	// remove outputs that were never used as input in this EvilBatch to determine batch outputs
	for output := range outputs {
		if _, ok := inputs[output]; ok {
			delete(outputs, output)
		}
	}
	batchOutputs = outputs
	return
}

// NextBatchPrefix creates a new batch prefix by increasing the number of created batches for this scenario.
func (e *EvilScenario) nextBatchPrefix() string {
	return e.ID + strconv.Itoa(int(e.BatchesCreated.Add(1)))
}

// ConflictBatchWithPrefix generates a new conflict batch with scenario prefix created from scenario ID and batch count.
// BatchOutputs are outputs of the batch that can be reused in deep spamming by collecting them in Reuse wallet.
func (e *EvilScenario) ConflictBatchWithPrefix() (prefixedBatch EvilBatch, allAliases ScenarioAlias, batchOutputs map[string]types.Empty) {
	allAliases = NewScenarioAlias()
	prefix := e.nextBatchPrefix()
	for _, conflictMap := range e.ConflictBatch {
		scenarioAlias := make([]ScenarioAlias, 0)
		for _, aliases := range conflictMap {
			sa := NewScenarioAlias()
			for _, in := range aliases.Inputs {
				sa.Inputs = append(sa.Inputs, prefix+in)
				allAliases.Inputs = append(allAliases.Inputs, prefix+in)
			}
			for _, out := range aliases.Outputs {
				sa.Outputs = append(sa.Outputs, prefix+out)
				allAliases.Outputs = append(allAliases.Outputs, prefix+out)
			}
			scenarioAlias = append(scenarioAlias, sa)
		}
		prefixedBatch = append(prefixedBatch, scenarioAlias)
	}
	batchOutputs = e.readCustomConflictsPattern(prefixedBatch)
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
