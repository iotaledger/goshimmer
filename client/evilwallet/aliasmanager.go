package evilwallet

import (
	"sync"

	"go.uber.org/atomic"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// region AliasManager /////////////////////////////////////////////////////////////////////////////////////////////////

// AliasManager is the manager for output aliases.
type AliasManager struct {
	outputMap map[string]ledgerstate.Output
	inputMap  map[string]ledgerstate.Input

	outputAliasCount *atomic.Uint64
	mu               sync.RWMutex
}

// NewAliasManager creates and returns a new AliasManager.
func NewAliasManager() *AliasManager {
	return &AliasManager{
		outputMap:        make(map[string]ledgerstate.Output),
		inputMap:         make(map[string]ledgerstate.Input),
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

// ClearAllAliases clears all aliases.
func (a *AliasManager) ClearAllAliases() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.inputMap = make(map[string]ledgerstate.Input)
	a.outputMap = make(map[string]ledgerstate.Output)
}

// ClearAliases clears provided aliases.
func (a *AliasManager) ClearAliases(aliases ScenarioAlias) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, in := range aliases.Inputs {
		delete(a.inputMap, in)
	}
	for _, out := range aliases.Outputs {
		delete(a.outputMap, out)
	}
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

// endregion /////////////////////////////////////////////////////////////////////////////////////////////////
