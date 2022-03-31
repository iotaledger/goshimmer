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

	waitForConfirmation = 60 * time.Second
	waitForTxSolid      = 2 * time.Second

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

// ClearAliases remove all registered alias names.
func (e *EvilWallet) ClearAliases() {
	e.aliasManager.ClearAliases()
}

func (e *EvilWallet) PrepareCustomConflicts(conflictsMaps []ConflictSlice, outputWallet *Wallet) (conflictBatch [][]*ledgerstate.Transaction, err error) {
	if outputWallet == nil {
		return nil, errors.Errorf("no output wallet provided")
	}
	for _, conflictMap := range conflictsMaps {
		var txs []*ledgerstate.Transaction
		for _, options := range conflictMap {
			options = append(options, WithOutputWallet(outputWallet))
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
	outputWallet := e.NewWallet()
	conflictBatch, err := e.PrepareCustomConflicts(conflictsMaps, outputWallet)
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
		time.Sleep(waitForTxSolid)
	}
	return
}

// CreateTransaction creates a transaction based on provided options. If no input wallet is provided, the next non-empty faucet wallet is used.
// Inputs of the transaction are determined in three ways:
// 1 - inputs are provided directly without associated alias, 2- alias is provided, and input is already stored in an alias manager,
// 3 - alias is provided, and there are no inputs assigned in Alias manager, so aliases are assigned to next ready inputs from input wallet.
func (e *EvilWallet) CreateTransaction(options ...Option) (tx *ledgerstate.Transaction, err error) {
	buildOptions := NewOptions(options...)

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
	outputs, addrAliasMap, err := e.prepareOutputs(buildOptions)
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

	err = e.updateOutputIDs(tx.ID(), tx.Essence().Outputs(), buildOptions.outputWallet)
	if err != nil {
		return nil, err
	}

	for _, o := range tx.Essence().Outputs() {
		e.outputManager.AddOutput(buildOptions.outputWallet, o)
	}

	e.registerOutputAliases(tx.Essence().Outputs(), addrAliasMap)

	return
}

func (e *EvilWallet) updateInputWallet(buildOptions *Options) error {
	// determine inputWallet based on the first input from AliasManager
	for alias := range buildOptions.aliasInputs {
		in, ok := e.aliasManager.GetInput(alias)
		if ok {
			err := e.updateIssuerWalletForAlias(buildOptions, in)
			if err != nil {
				return err
			}
			return nil
		}
		break
	}
	// if input wallet is not specified, use Fresh faucet wallet
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

func (e *EvilWallet) prepareOutputs(buildOptions *Options) (outputs []ledgerstate.Output, addrAliasMap map[ledgerstate.Address]string, err error) {
	// if outputs were provided with aliases
	if !buildOptions.areOutputsProvidedWithoutAliases() {
		outputs, addrAliasMap, err = e.matchOutputsWithAliases(buildOptions)
	} else {
		for _, balance := range buildOptions.outputs {
			evilOutput := e.outputManager.CreateEmptyOutput(buildOptions.outputWallet, balance)
			output := ledgerstate.NewSigLockedColoredOutput(balance, evilOutput.Address)
			if err != nil {
				return nil, nil, err
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
		if wallet, err := e.wallets.FreshWallet(); wallet != nil {
			buildOptions.inputWallet = wallet
		} else {
			return errors.Newf("no Fresh wallet is available: %w", err)
		}
	}
	return nil
}

func (e *EvilWallet) updateIssuerWalletForAlias(buildOptions *Options, in ledgerstate.Input) error {
	inputWallet := e.outputManager.OutputIDWalletMap(in.Base58())
	if buildOptions.inputWallet == nil {
		buildOptions.inputWallet = inputWallet
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
	for alias, balance := range buildOptions.aliasOutputs {
		evilOutput := e.outputManager.CreateEmptyOutput(buildOptions.outputWallet, balance)
		output := ledgerstate.NewSigLockedColoredOutput(balance, evilOutput.Address)

		outputs = append(outputs, output)
		addrAliasMap[evilOutput.Address] = alias
	}
	for _, balance := range buildOptions.outputs {
		evilOutput := e.outputManager.CreateEmptyOutput(buildOptions.outputWallet, balance)
		output := ledgerstate.NewSigLockedColoredOutput(balance, evilOutput.Address)
		if err != nil {
			return nil, nil, err
		}
		outputs = append(outputs, output)
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
		unlockBlocks[i] = ledgerstate.NewSignatureUnlockBlock(w.Sign(addr, txEssence))
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
	for _, output := range outputs {
		err := e.outputManager.UpdateOutputID(outWallet, output.Address().Base58(), output.ID())
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *EvilWallet) PrepareTransaction(scenario *EvilScenario) (tx *ledgerstate.Transaction, err error) {
	var wallet *Wallet
	var evilInput *Output
	if scenario.Reuse {
		wallet, err = e.wallets.reuseWallet()
		if err != nil {
			return
		}
		if wallet != nil {
			evilInput = wallet.GetUnspentOutput()
		}
	}
	if evilInput == nil {
		wallet, err = e.wallets.freshWallet()
		if err != nil {
			return
		}
		evilInput = wallet.GetUnspentOutput()
	}
	outBalance := getIotaColorAmount(evilInput.Balance)
	tx, err = e.CreateTransaction(WithInputs(evilInput.OutputID), WithOutputs([]*OutputOption{{amount: outBalance}}), WithOutputWallet(scenario.OutputWallet), WithIssuer(wallet))
	return
}

func (e *EvilWallet) PrepareCustomConflictsSpam(scenario *EvilScenario) (txs [][]*ledgerstate.Transaction, err error) {
	conflicts, err := e.prepareConflictSliceForScenario(scenario)
	if err != nil {
		return nil, err
	}
	scenario.conflictSlice = conflicts
	txs, err = e.PrepareCustomConflicts(scenario.conflictSlice, scenario.OutputWallet)

	return
}

func (e *EvilWallet) prepareConflictSliceForScenario(scenario *EvilScenario) (conflicts []ConflictSlice, err error) {
	inWallet, err := e.wallets.GetNextWallet(Fresh)
	if err != nil {
		return nil, err
	}

	// Register the alias name for the unspent outputs from inWallet.
	for in := range scenario.inputsAlias {
		unspentOutput := inWallet.GetUnspentOutput()
		input := ledgerstate.NewUTXOInput(unspentOutput.OutputID)
		e.aliasManager.AddInputAlias(input, in)
	}

	genOutputOptions := func(aliases []string) []*OutputOption {
		outputOptions := make([]*OutputOption, 0)
		for _, o := range aliases {
			outputOptions = append(outputOptions, &OutputOption{aliasName: o})
		}
		return outputOptions
	}

	// make conflictSlice
	conflictSlice := make([]ConflictSlice, 0)
	for _, conflictMap := range scenario.ConflictBatch {
		conflicts := make([][]Option, 0)
		for _, aliases := range conflictMap {
			outs := genOutputOptions(aliases.Outputs)
			conflicts = append(conflicts, []Option{WithInputs(aliases.Inputs), WithOutputs(outs), WithIssuer(inWallet)})
		}
		conflictSlice = append(conflictSlice, conflicts)
	}

	return conflictSlice, nil
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

type EvilBatch [][]ScenarioAlias

type EvilScenario struct {
	ConflictBatch EvilBatch
	conflictSlice []ConflictSlice
	// determines whether outputs of the batch  should be reused during the spam to create deep UTXO tree structure.
	Reuse bool
	// if provided, the outputs from the spam will be saved into this wallet, accepted types of wallet: Reuse, RestrictedReuse.
	// if type == Reuse, then wallet is available for reuse spamming scenarios that did not provide RestrictedWallet.
	OutputWallet *Wallet
	// if provided and reuse set to true, outputs from this wallet will be used for deep spamming, allows for controllable building of UTXO deep structures.
	// if not provided evil wallet will use Reuse wallet if any is available. Accepts only RestrictedReuse wallet type.
	RestrictedInputWallet *Wallet

	// outputs of the batch that can be reused in deep spamming by collecting them in Reuse wallet.
	batchOutputsAliases map[string]types.Empty
	inputsAlias         map[string]types.Empty
}

func NewEvilScenario(options ...ScenarioOption) *EvilScenario {
	scenario := &EvilScenario{
		ConflictBatch: SingleTransactionBatch(),
		Reuse:         false,
		OutputWallet:  NewWallet(),
	}

	for _, option := range options {
		option(scenario)
	}

	scenario.assignBatchOutputs()
	return scenario
}

func (e *EvilScenario) assignBatchOutputs() {
	e.batchOutputsAliases = make(map[string]types.Empty)
	for _, conflictMap := range e.ConflictBatch {
		for _, options := range conflictMap {
			option := NewOptions(options...)
			for outputAlis := range option.aliasOutputs {
				// add output aliases that are not used in this conflict batch
				if _, ok := option.aliasInputs[outputAlis]; !ok {
					e.batchOutputsAliases[outputAlis] = types.Void
				}
			}
		}
	}
}

func (e *EvilScenario) readCustomConflictsPattern() {
	e.batchOutputsAliases = make(map[string]types.Empty)
	e.inputsAlias = make(map[string]types.Empty)
	deleteFromOutput := make(map[string]types.Empty)

	for _, conflictMap := range e.ConflictBatch {
		for _, aliases := range conflictMap {
			// add output to batchOutputsAliases
			for _, output := range aliases.Outputs {
				if _, ok := e.batchOutputsAliases[output]; !ok {
					e.batchOutputsAliases[output] = types.Void
				}
			}
			// add input only if it's not in output, this will determine how many
			// unspent outputs to take in each round of spamming.
			for _, input := range aliases.Inputs {
				if _, ok := e.batchOutputsAliases[input]; !ok {
					e.inputsAlias[input] = types.Void
				} else {
					deleteFromOutput[input] = types.Void
				}
			}
		}
	}

	for d := range deleteFromOutput {
		delete(e.batchOutputsAliases, d)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
