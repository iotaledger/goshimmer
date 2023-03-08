package evilwallet

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/types"
)

const (
	// FaucetRequestSplitNumber defines the number of outputs to split from a faucet request.
	FaucetRequestSplitNumber = 100
	faucetTokensPerRequest   = 1000000

	waitForConfirmation   = 150 * time.Second
	waitForSolidification = 150 * time.Second

	awaitConfirmationSleep   = 3 * time.Second
	awaitSolidificationSleep = time.Millisecond * 500

	WaitForTxSolid = 150 * time.Second

	maxGoroutines = 5
)

var (
	defaultClientsURLs = []string{}
	faucetBalance      = devnetvm.NewColoredBalances(map[devnetvm.Color]uint64{
		devnetvm.ColorIOTA: uint64(faucetTokensPerRequest),
	})
)

// region EvilWallet ///////////////////////////////////////////////////////////////////////////////////////////////////////

// EvilWallet provides a user-friendly way to do complicated double spend scenarios.
type EvilWallet struct {
	wallets       *Wallets
	connector     Connector
	outputManager *OutputManager
	aliasManager  *AliasManager
}

// NewEvilWallet creates an EvilWallet instance.
func NewEvilWallet(clientsURLs ...string) *EvilWallet {
	urls := clientsURLs
	if len(urls) == 0 {
		urls = append(urls, defaultClientsURLs...)
	}

	connector := NewWebClients(urls)
	wallets := NewWallets()
	return &EvilWallet{
		wallets:       wallets,
		connector:     connector,
		outputManager: NewOutputManager(connector, wallets),
		aliasManager:  NewAliasManager(),
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

func (e *EvilWallet) UnspentOutputsLeft(walletType WalletType) int {
	return e.wallets.UnspentOutputsLeft(walletType)
}

func (e *EvilWallet) NumOfClient() int {
	clts := e.connector.Clients()
	return len(clts)
}

func (e *EvilWallet) AddClient(clientURL string) {
	e.connector.AddClient(clientURL)
}

func (e *EvilWallet) RemoveClient(clientURL string) {
	e.connector.RemoveClient(clientURL)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region EvilWallet Faucet Requests ///////////////////////////////////////////////////////////////////////////////////

// RequestFundsFromFaucet requests funds from the faucet, then track the confirmed status of unspent output,
// also register the alias name for the unspent output if provided.
func (e *EvilWallet) RequestFundsFromFaucet(options ...FaucetRequestOption) (initWallet *Wallet, err error) {
	initWallet = e.NewWallet(Fresh)
	buildOptions := NewFaucetRequestOptions(options...)

	outputID, err := e.requestFaucetFunds(initWallet)
	if err != nil {
		return
	}

	if buildOptions.outputAliasName != "" {
		input := devnetvm.NewUTXOInput(outputID)
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
	fmt.Println("Requesting funds from faucet...")
	funds, err := e.requestAndSplitFaucetFunds(initWallet, FaucetRequestSplitNumber*FaucetRequestSplitNumber)
	if err != nil {
		return
	}
	e.wallets.SetWalletReady(funds)
	return
}

// RequestFreshFaucetWallet creates a new wallet and fills the wallet with 100 outputs created from funds
// requested from the Faucet.
func (e *EvilWallet) RequestFreshFaucetWallet() (err error) {
	initWallet := NewWallet()
	wallet, err := e.requestAndSplitFaucetFunds(initWallet, FaucetRequestSplitNumber)
	if err != nil {
		return
	}
	e.wallets.SetWalletReady(wallet)
	return
}

func (e *EvilWallet) requestAndSplitFaucetFunds(initWallet *Wallet, splitNum int) (wallet *Wallet, err error) {
	_, err = e.requestFaucetFunds(initWallet)
	if err != nil {
		return
	}
	// first split 1 to FaucetRequestSplitNumber outputs
	wallet = e.NewWallet(Fresh)
	err = e.splitOutputs(initWallet, wallet, splitNum)

	return
}

func (e *EvilWallet) requestFaucetFunds(wallet *Wallet) (outputID utxo.OutputID, err error) {
	addr := wallet.Address()
	clt := e.connector.GetClient()
	if err = RateSetterSleep(clt, true); err != nil {
		return
	}
	err = clt.BroadcastFaucetRequest(addr.Base58(), 12)
	if err != nil {
		return
	}
	output := e.outputManager.CreateOutputFromAddress(wallet, addr, faucetBalance)
	if output == nil {
		err = errors.New("could not get output from a given address")
		return
	}
	// track output in output manager and make sure it's confirmed
	ok := e.outputManager.Track([]utxo.OutputID{output.OutputID})
	if !ok {
		err = errors.New("not all outputs has been confirmed")
		return
	}
	outputID = output.OutputID
	return
}

func (e *EvilWallet) splitOutputs(inputWallet, outputWallet *Wallet, splitNumber int) error {
	var txIDs []string
	wg := sync.WaitGroup{}

	if inputWallet.IsEmpty() {
		return errors.New("inputWallet is empty")
	}

	// we split 100 outputs in each round
	round := 0
	for i := splitNumber; i > 1; round++ {
		i /= FaucetRequestSplitNumber
	}

	var tmpOutWallet, tmpInWallet *Wallet
	for i := 0; i < round; i++ {
		// prepare wallet for next round, new tmpOutWallet are not managed by evil wallet
		if i == 0 {
			tmpInWallet = inputWallet
		} else {
			tmpInWallet = tmpOutWallet
		}

		if i == round-1 {
			tmpOutWallet = outputWallet
		} else {
			tmpOutWallet = NewWallet(Fresh)
		}

		txIDs = make([]string, int(math.Pow(FaucetRequestSplitNumber, float64(i))))
		inputNum := 0
		for addr := range tmpInWallet.UnspentOutputs() {
			wg.Add(1)
			go func(inputNum int, addr string) {
				defer wg.Done()

				input, outputs := e.handleInputOutputDuringSplitOutputs(FaucetRequestSplitNumber, tmpInWallet, addr)

				tx, err := e.CreateTransaction(WithInputs(input), WithOutputs(outputs),
					WithIssuer(tmpInWallet), WithOutputWallet(tmpOutWallet))
				if err != nil {
					return
				}

				clt := e.connector.GetClient()
				if err = RateSetterSleep(clt, true); err != nil {
					return
				}
				txID, _, err := clt.PostTransaction(tx)
				if err != nil {
					return
				}

				txIDs[inputNum] = txID.Base58()
			}(inputNum, addr)
			inputNum++
		}
		wg.Wait()

		// wait txs to be confirmed in each round
		e.outputManager.AwaitTransactionsConfirmation(txIDs, maxGoroutines)
	}

	return nil
}

func (e *EvilWallet) handleInputOutputDuringSplitOutputs(splitNumber int, inputWallet *Wallet, inputAddr string) (input utxo.OutputID, outputs []*OutputOption) {
	evilInput := inputWallet.UnspentOutput(inputAddr)
	input = evilInput.OutputID

	inputBalance := uint64(0)
	evilInput.Balance.ForEach(func(color devnetvm.Color, balance uint64) bool {
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

func (e *EvilWallet) PrepareCustomConflicts(conflictsMaps []ConflictSlice) (conflictBatch [][]*devnetvm.Transaction, err error) {
	for _, conflictMap := range conflictsMaps {
		var txs []*devnetvm.Transaction
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
			go func(clt Client, tx *devnetvm.Transaction) {
				defer wg.Done()
				if err = RateSetterSleep(clt, true); err != nil {
					return
				}
				_, _, _ = clt.PostTransaction(tx)
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
func (e *EvilWallet) CreateTransaction(options ...Option) (tx *devnetvm.Transaction, err error) {
	buildOptions, err := NewOptions(options...)
	if err != nil {
		return nil, err
	}
	// wallet used only for outputs in the middle of the batch, that will never be reused outside custom conflict batch creation.
	tempWallet := e.NewWallet()

	err = e.updateInputWallet(buildOptions)
	if err != nil {
		return nil, err
	}

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

	tx, err = e.makeTransaction(devnetvm.NewInputs(inputs...), devnetvm.NewOutputs(outputs...), buildOptions.inputWallet)
	if err != nil {
		return nil, err
	}

	e.addOutputsToOutputManager(tx, buildOptions.outputWallet, tempWallet, tempAddresses)
	e.registerOutputAliases(tx.Essence().Outputs(), addrAliasMap)

	return
}

// addOutputsToOutputManager adds output to the OutputManager if.
func (e *EvilWallet) addOutputsToOutputManager(tx *devnetvm.Transaction, outWallet, tmpWallet *Wallet, tempAddresses map[devnetvm.Address]types.Empty) {
	for _, o := range tx.Essence().Outputs() {
		if _, ok := tempAddresses[o.Address()]; ok {
			e.outputManager.AddOutput(tmpWallet, o)
		} else {
			e.outputManager.AddOutput(outWallet, o)
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
	wallet, err := e.useFreshIfInputWalletNotProvided(buildOptions)
	if err != nil {
		return err
	}
	buildOptions.inputWallet = wallet
	return nil
}

func (e *EvilWallet) registerOutputAliases(outputs devnetvm.Outputs, addrAliasMap map[devnetvm.Address]string) {
	if len(addrAliasMap) == 0 {
		return
	}

	for _, output := range outputs {
		// register output alias
		e.aliasManager.AddOutputAlias(output, addrAliasMap[output.Address()])

		// register output as unspent output(input)
		input := devnetvm.NewUTXOInput(output.ID())
		e.aliasManager.AddInputAlias(input, addrAliasMap[output.Address()])
	}
}

func (e *EvilWallet) prepareInputs(buildOptions *Options) (inputs []devnetvm.Input, err error) {
	if buildOptions.areInputsProvidedWithoutAliases() {
		for _, out := range buildOptions.inputs {
			inputs = append(inputs, devnetvm.NewUTXOInput(out))
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
func (e *EvilWallet) prepareOutputs(buildOptions *Options, tempWallet *Wallet) (outputs []devnetvm.Output,
	addrAliasMap map[devnetvm.Address]string, tempAddresses map[devnetvm.Address]types.Empty, err error,
) {
	if buildOptions.areOutputsProvidedWithoutAliases() {
		for _, balance := range buildOptions.outputs {
			output := devnetvm.NewSigLockedColoredOutput(balance, buildOptions.outputWallet.Address().Address())
			outputs = append(outputs, output)
		}
	} else {
		// if outputs were provided with aliases
		outputs, addrAliasMap, tempAddresses, err = e.matchOutputsWithAliases(buildOptions, tempWallet)
	}
	return
}

// matchInputsWithAliases gets input from the alias manager. if input was not assigned to an alias before,
// it assigns a new Fresh faucet output.
func (e *EvilWallet) matchInputsWithAliases(buildOptions *Options) (inputs []devnetvm.Input, err error) {
	// get inputs by alias
	for inputAlias := range buildOptions.aliasInputs {
		in, ok := e.aliasManager.GetInput(inputAlias)
		if !ok {
			wallet, err2 := e.useFreshIfInputWalletNotProvided(buildOptions)
			if err2 != nil {
				err = err2
				return
			}
			// No output found for given alias, use internal Fresh output if wallets are non-empty.
			out := e.wallets.GetUnspentOutput(wallet)
			if out == nil {
				return nil, errors.New("could not get unspent output")
			}
			in = devnetvm.NewUTXOInput(out.OutputID)
			e.aliasManager.AddInputAlias(in, inputAlias)
		}
		inputs = append(inputs, in)
	}
	return inputs, nil
}

func (e *EvilWallet) useFreshIfInputWalletNotProvided(buildOptions *Options) (*Wallet, error) {
	// if input wallet is not specified, use Fresh faucet wallet
	if buildOptions.inputWallet == nil {
		// deep spam enabled and no input reuse wallet provided, use evil wallet reuse wallet if enough outputs are available
		if buildOptions.reuse {
			outputsNeeded := len(buildOptions.inputs)
			if wallet := e.wallets.reuseWallet(outputsNeeded); wallet != nil {
				return wallet, nil
			}
		}
		if wallet, err := e.wallets.freshWallet(); wallet != nil {
			return wallet, nil
		} else {
			return nil, errors.Wrap(err, "no Fresh wallet is available")
		}
	}
	return buildOptions.inputWallet, nil
}

// matchOutputsWithAliases creates outputs based on balances provided via options.
// Outputs are not yet added to the Alias Manager, as they have no ID before the transaction is created.
// Thus, they are tracker in address to alias map. If the scenario is used, the outputBatchAliases map is provided
// that indicates which outputs should be saved to the outputWallet.All other outputs are created with temporary wallet,
// and their addresses are stored in tempAddresses.
func (e *EvilWallet) matchOutputsWithAliases(buildOptions *Options, tempWallet *Wallet) (outputs []devnetvm.Output,
	addrAliasMap map[devnetvm.Address]string, tempAddresses map[devnetvm.Address]types.Empty, err error,
) {
	err = e.updateOutputBalances(buildOptions)
	if err != nil {
		return nil, nil, nil, err
	}

	tempAddresses = make(map[devnetvm.Address]types.Empty)
	addrAliasMap = make(map[devnetvm.Address]string)
	for alias, balance := range buildOptions.aliasOutputs {
		var addr devnetvm.Address
		if _, ok := buildOptions.outputBatchAliases[alias]; ok {
			addr = buildOptions.outputWallet.Address().Address()
		} else {
			addr = tempWallet.Address().Address()
			tempAddresses[addr] = types.Void
		}

		outputs = append(outputs, devnetvm.NewSigLockedColoredOutput(balance, addr))
		addrAliasMap[addr] = alias
	}

	return
}

func (e *EvilWallet) prepareRemainderOutput(buildOptions *Options, outputs []devnetvm.Output) (alias string, remainderOutput devnetvm.Output, added bool) {
	inputBalance := uint64(0)

	var remainderAddress devnetvm.Address
	for inputAlias := range buildOptions.aliasInputs {
		in, _ := e.aliasManager.GetInput(inputAlias)
		// get balance from output manager
		var out utxo.OutputID
		if err := out.FromBase58(in.Base58()); err != nil {
			panic(err)
		}
		output := e.outputManager.GetOutput(out)

		output.Balance.ForEach(func(color devnetvm.Color, balance uint64) bool {
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
		inputDetails.Balance.ForEach(func(color devnetvm.Color, balance uint64) bool {
			inputBalance += balance
			return true
		})
		if remainderAddress == address.AddressEmpty.Address() {
			remainderAddress = inputDetails.Address
		}
	}

	outputBalance := uint64(0)
	for _, o := range outputs {
		o.Balances().ForEach(func(color devnetvm.Color, balance uint64) bool {
			outputBalance += balance
			return true
		})
	}

	// remainder balances is sent to one of the address in inputs
	if outputBalance < inputBalance {
		remainderOutput = devnetvm.NewSigLockedColoredOutput(devnetvm.NewColoredBalances(map[devnetvm.Color]uint64{
			devnetvm.ColorIOTA: inputBalance - outputBalance,
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
				inputDetails.Balance.ForEach(func(color devnetvm.Color, balance uint64) bool {
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
				var out utxo.OutputID
				if err := out.FromBase58(in.Base58()); err != nil {
					panic(err)
				}
				output := e.outputManager.GetOutput(out)
				output.Balance.ForEach(func(color devnetvm.Color, balance uint64) bool {
					totalBalance += balance
					return true
				})
			}
		}
		balances := SplitBalanceEqually(len(buildOptions.outputs)+len(buildOptions.aliasOutputs), totalBalance)
		i := 0
		for out := range buildOptions.aliasOutputs {
			buildOptions.aliasOutputs[out] = devnetvm.NewColoredBalances(map[devnetvm.Color]uint64{
				devnetvm.ColorIOTA: balances[i],
			})
			i++
		}
	}
	return
}

func (e *EvilWallet) makeTransaction(inputs devnetvm.Inputs, outputs devnetvm.Outputs, w *Wallet) (tx *devnetvm.Transaction, err error) {
	txEssence := devnetvm.NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, inputs, outputs)
	unlockBlocks := make([]devnetvm.UnlockBlock, len(txEssence.Inputs()))
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
		unlockBlocks[i] = devnetvm.NewSignatureUnlockBlock(wallet.Sign(addr, txEssence))
	}
	return devnetvm.NewTransaction(txEssence, unlockBlocks), nil
}

func (e *EvilWallet) getAddressFromInput(input devnetvm.Input) (addr devnetvm.Address, err error) {
	typeCastedInput, ok := input.(*devnetvm.UTXOInput)
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

func (e *EvilWallet) PrepareCustomConflictsSpam(scenario *EvilScenario) (txs [][]*devnetvm.Transaction, allAliases ScenarioAlias, err error) {
	conflicts, allAliases := e.prepareConflictSliceForScenario(scenario)
	txs, err = e.PrepareCustomConflicts(conflicts)

	return
}

func (e *EvilWallet) prepareConflictSliceForScenario(scenario *EvilScenario) (conflictSlice []ConflictSlice, allAliases ScenarioAlias) {
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
			option := []Option{WithInputs(aliases.Inputs), WithOutputs(outs), WithOutputBatchAliases(batchOutputs)}
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

// AwaitInputsSolidity waits for all inputs to be solid for client clt.
func (e *EvilWallet) AwaitInputsSolidity(inputs devnetvm.Inputs, clt Client) (allSolid bool) {
	awaitSolid := make([]string, 0)
	for _, in := range inputs {
		awaitSolid = append(awaitSolid, in.Base58())
	}
	allSolid = e.outputManager.AwaitOutputsToBeSolid(awaitSolid, clt, maxGoroutines)
	return
}

// SetTxOutputsSolid marks all outputs as solid in OutputManager for clientID.
func (e *EvilWallet) SetTxOutputsSolid(outputs devnetvm.Outputs, clientID string) {
	for _, out := range outputs {
		e.outputManager.SetOutputIDSolidForIssuer(out.ID().Base58(), clientID)
	}
}

// AddReuseOutputsToThePool adds all addresses corresponding to provided outputs to the reuse pool.
func (e *EvilWallet) AddReuseOutputsToThePool(outputs devnetvm.Outputs) {
	for _, out := range outputs {
		evilOutput := e.outputManager.GetOutput(out.ID())
		if evilOutput != nil {
			wallet := e.outputManager.OutputIDWalletMap(out.ID().Base58())
			wallet.AddReuseAddress(evilOutput.Address.Base58())
		}
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
