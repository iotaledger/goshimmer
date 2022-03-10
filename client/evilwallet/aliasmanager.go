package evilwallet

import (
	"errors"
	"fmt"
	"go.uber.org/atomic"
	"sync"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// region AliasManager /////////////////////////////////////////////////////////////////////////////////////////////////

type AliasManager struct {
	outputMap map[string]ledgerstate.Output
	inputMap  map[string]ledgerstate.Input
	txMap     map[string]*ledgerstate.Transaction

	outputAliasCount *atomic.Uint64
	mu               sync.RWMutex
}

func NewAliasManager() *AliasManager {
	return &AliasManager{
		outputMap:        make(map[string]ledgerstate.Output),
		inputMap:         make(map[string]ledgerstate.Input),
		txMap:            make(map[string]*ledgerstate.Transaction),
		outputAliasCount: atomic.NewUint64(0),
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

func (a *AliasManager) GetInput(aliasName string) (ledgerstate.Input, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	in, ok := a.inputMap[aliasName]
	return in, ok
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

func (a *AliasManager) AddOutputAliases(outputs []ledgerstate.Output, aliases []string) {
	for i, out := range outputs {
		err := a.AddOutputAlias(out, aliases[i])
		if err != nil {
			return
		}
	}
	return
}

func (a *AliasManager) AddInputAliases(inputs []*Output, aliases []string) {
	for i, out := range inputs {
		input := ledgerstate.NewUTXOInput(out.OutputID)
		err := a.AddInputAlias(input, aliases[i])
		if err != nil {
			return
		}
	}
	return
}

func (a *AliasManager) CreateAliasForTransaction(outWalletID, inWalletID int, outID string) string {
	aliasName := fmt.Sprintf("txO%dI%dOut%s", outWalletID, inWalletID, outID)
	return aliasName
}

func (a *AliasManager) CreateAliasesForOutputs(walletID, aliasesNum int) (aliases []string) {
	for i := 0; i < aliasesNum; i++ {
		aliases = append(aliases, a.createAliasForOutput(walletID))
	}
	return
}

func (a *AliasManager) createAliasForOutput(walletID int) string {
	return fmt.Sprintf("outW%dCount%d", walletID, a.outputAliasCount.Add(1))
}

func (a *AliasManager) createAliasForInput() string {
	aliasName := fmt.Sprintf("InputCount%d", a.outputAliasCount.Add(1))
	return aliasName
}

func (a *AliasManager) CreateAliasesForInputs(aliasesNum int) (aliases []string) {
	for i := 0; i < aliasesNum; i++ {
		aliases = append(aliases, a.createAliasForInput())
	}
	return
}

// endregion /////////////////////////////////////////////////////////////////////////////////////////////////
