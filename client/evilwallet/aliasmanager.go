package evilwallet

import (
	"errors"
	"fmt"
	"sync"

	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// region AliasManager /////////////////////////////////////////////////////////////////////////////////////////////////

// AliasManager is the manager for output aliases.
type AliasManager struct {
	outputMap map[string]ledgerstate.Output
	inputMap  map[string]ledgerstate.Input
	txMap     map[string]*ledgerstate.Transaction

	outputAliasCount *atomic.Uint64
	mu               sync.RWMutex
}

// NewAliasManager creates and returns a new AliasManager.
func NewAliasManager() *AliasManager {
	return &AliasManager{
		outputMap:        make(map[string]ledgerstate.Output),
		inputMap:         make(map[string]ledgerstate.Input),
		txMap:            make(map[string]*ledgerstate.Transaction),
		outputAliasCount: atomic.NewUint64(0),
	}
}

// AddOutputAlias maps the given outputAliasName to output, if there's duplicate outputAliasName, it will be overwritten.
func (a *AliasManager) AddOutputAlias(output ledgerstate.Output, aliasName string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.outputMap[aliasName] = output
	return
}

// AddInputAlias adds an input alias.
func (a *AliasManager) AddInputAlias(input ledgerstate.Input, aliasName string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.inputMap[aliasName] = input
	return
}

// AddTransactionAlias adds a transaction alias.
func (a *AliasManager) AddTransactionAlias(tx *ledgerstate.Transaction, aliasName string) (err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.txMap[aliasName]; exists {
		err = errors.New("duplicate alias name in input alias")
		return
	}

	a.txMap[aliasName] = tx
	return nil
}

// GetInput returns the input for the alias specified.
func (a *AliasManager) GetInput(aliasName string) (ledgerstate.Input, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	in, ok := a.inputMap[aliasName]
	return in, ok
}

// GetOutput returns the output for the alias specified.
func (a *AliasManager) GetOutput(aliasName string) ledgerstate.Output {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.outputMap[aliasName]
}

// ClearAliases clears all aliases.
func (a *AliasManager) ClearAliases() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.inputMap = make(map[string]ledgerstate.Input)
	a.outputMap = make(map[string]ledgerstate.Output)
	a.txMap = make(map[string]*ledgerstate.Transaction)
}

// AddOutputAliases batch adds the outputs their respective aliases.
func (a *AliasManager) AddOutputAliases(outputs []ledgerstate.Output, aliases []string) error {
	if len(outputs) != len(aliases) {
		return errors.New("mismatch outputs and aliases length")
	}
	for i, out := range outputs {
		a.AddOutputAlias(out, aliases[i])
	}
	return nil
}

// AddInputAliases batch adds the inputs their respective aliases.
func (a *AliasManager) AddInputAliases(inputs []*Output, aliases []string) error {
	if len(inputs) != len(aliases) {
		return errors.New("mismatch outputs and aliases length")
	}
	for i, out := range inputs {
		input := ledgerstate.NewUTXOInput(out.OutputID)
		a.AddInputAlias(input, aliases[i])
	}
	return nil
}

// CreateAliasForTransaction creates an alias for the transaction.
func (a *AliasManager) CreateAliasForTransaction(outWalletID, inWalletID walletID) string {
	aliasName := fmt.Sprintf("txO%dI%dCount%d", outWalletID, inWalletID, a.outputAliasCount.Add(1))
	return aliasName
}

// CreateAliasesForTransactions creates `aliasesNum` transaction aliases.
func (a *AliasManager) CreateAliasesForTransactions(aliasesNum int, outWalletID, inWalletID walletID) (aliases []string) {
	for i := 0; i < aliasesNum; i++ {
		aliases = append(aliases, a.CreateAliasForTransaction(outWalletID, inWalletID))
	}
	return
}

// CreateAliasesForOutputs creates `aliasesNum` output aliases.
func (a *AliasManager) CreateAliasesForOutputs(walletID walletID, aliasesNum int) (aliases []string) {
	for i := 0; i < aliasesNum; i++ {
		aliases = append(aliases, a.createAliasForOutput(walletID))
	}
	return
}

func (a *AliasManager) createAliasForOutput(walletID walletID) string {
	return fmt.Sprintf("outW%dCount%d", walletID, a.outputAliasCount.Add(1))
}

func (a *AliasManager) createAliasForInput() string {
	aliasName := fmt.Sprintf("InputCount%d", a.outputAliasCount.Add(1))
	return aliasName
}

// CreateAliasesForInputs creates `aliasesNum` input aliases.
func (a *AliasManager) CreateAliasesForInputs(aliasesNum int) (aliases []string) {
	for i := 0; i < aliasesNum; i++ {
		aliases = append(aliases, a.createAliasForInput())
	}
	return
}

// endregion /////////////////////////////////////////////////////////////////////////////////////////////////
