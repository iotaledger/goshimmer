package evilwallet

import (
	"sync"

	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// region AliasManager /////////////////////////////////////////////////////////////////////////////////////////////////

type AliasManager struct {
	outputMap map[string]ledgerstate.Output
	inputMap  map[string]ledgerstate.Input

	outputAliasCount *atomic.Uint64
	mu               sync.RWMutex
}

func NewAliasManager() *AliasManager {
	return &AliasManager{
		outputMap:        make(map[string]ledgerstate.Output),
		inputMap:         make(map[string]ledgerstate.Input),
		outputAliasCount: atomic.NewUint64(0),
	}
}

// AddOutputAlias maps the given aliasName to output, if there's duplicate aliasName, it will be overwritten.
func (a *AliasManager) AddOutputAlias(output ledgerstate.Output, aliasName string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.outputMap[aliasName] = output
	return
}

func (a *AliasManager) AddInputAlias(input ledgerstate.Input, aliasName string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.inputMap[aliasName] = input
	return
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
}

func (a *AliasManager) AddOutputAliases(outputs []ledgerstate.Output, aliases []string) {
	for i, out := range outputs {
		a.AddOutputAlias(out, aliases[i])
	}
	return
}

func (a *AliasManager) AddInputAliases(inputs []*Output, aliases []string) {
	for i, out := range inputs {
		input := ledgerstate.NewUTXOInput(out.OutputID)
		a.AddInputAlias(input, aliases[i])
	}
	return
}

// endregion /////////////////////////////////////////////////////////////////////////////////////////////////
