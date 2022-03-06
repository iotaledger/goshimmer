package evilwallet

import (
	"errors"
	"fmt"
	"sync"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// region AliasManager /////////////////////////////////////////////////////////////////////////////////////////////////

type AliasManager struct {
	outputMap map[string]ledgerstate.Output
	inputMap  map[string]ledgerstate.Input
	txMap     map[string]*ledgerstate.Transaction

	mu sync.RWMutex
}

func NewAliasManager() *AliasManager {
	return &AliasManager{
		outputMap: make(map[string]ledgerstate.Output),
		inputMap:  make(map[string]ledgerstate.Input),
		txMap:     make(map[string]*ledgerstate.Transaction),
	}
}

func (a *AliasManager) AddOutputAlias(output ledgerstate.Output, aliasName string) (err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.outputMap[aliasName]; exists {
		err = errors.New("duplicate alias name in output alias")
		return
	}

	a.outputMap[aliasName] = output
	return nil
}

func (a *AliasManager) AddInputAlias(input ledgerstate.Input, aliasName string) (err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.inputMap[aliasName]; exists {
		err = errors.New("duplicate alias name in input alias")
		return
	}

	a.inputMap[aliasName] = input
	return nil
}

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

func (a *AliasManager) GetInput(aliasName string) ledgerstate.Input {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.inputMap[aliasName]
}

func (a *AliasManager) GetOutput(aliasName string) ledgerstate.Output {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.outputMap[aliasName]
}

func (a *AliasManager) ClearAliases() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.inputMap = make(map[string]ledgerstate.Input)
	a.outputMap = make(map[string]ledgerstate.Output)
	a.txMap = make(map[string]*ledgerstate.Transaction)
}

// endregion /////////////////////////////////////////////////////////////////////////////////////////////////

// region InternalAliasManager /////////////////////////////////////////////////////////////////////////////////////////////////

// InternalAliasManager extends AliasManager functionality to handel automated tasks during an EvilScenarios creation.
type InternalAliasManager struct {
	*AliasManager
}

func NewEvilAliasManager() *InternalAliasManager {
	return &InternalAliasManager{
		NewAliasManager(),
	}
}

func (a *InternalAliasManager) CreateOutputAlias(output ledgerstate.Output) string {
	aliasName := fmt.Sprintf("out%s", output.ID().Base58())
	err := a.AddOutputAlias(output, aliasName)
	if err != nil {
		return ""
	}
	return aliasName
}

func (a *InternalAliasManager) CreateInputAlias(input ledgerstate.Input) string {
	aliasName := fmt.Sprintf("in%s", input.Base58())
	err := a.AddInputAlias(input, aliasName)
	if err != nil {
		return ""
	}
	return aliasName
}

func (a *InternalAliasManager) CreateTransactionAlias(tx *ledgerstate.Transaction) string {
	aliasName := fmt.Sprintf("tx%s", tx.ID().Base58())
	err := a.AddTransactionAlias(tx, aliasName)
	if err != nil {
		return ""
	}
	return aliasName
}

func (a *InternalAliasManager) CreateAliasesForOutputs(outputs ledgerstate.Outputs) (aliases []string) {
	for _, out := range outputs {
		aliases = append(aliases, a.CreateOutputAlias(out))
	}
	return
}

func (a *InternalAliasManager) CreateAliasesForInputs(inputs []*Output) (aliases []string) {
	for _, in := range inputs {
		input := ledgerstate.NewUTXOInput(in.OutputID)
		aliases = append(aliases, a.CreateInputAlias(input))
	}
	return
}

// endregion /////////////////////////////////////////////////////////////////////////////////////////////////
