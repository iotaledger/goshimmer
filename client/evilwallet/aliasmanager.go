package evilwallet

import (
	"errors"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

type AliasManager struct {
	outputMap map[string]ledgerstate.Output
	inputMap  map[string]ledgerstate.Input
	txMap     map[string]*ledgerstate.Transaction
}

func NewAliasManager() *AliasManager {
	return &AliasManager{
		outputMap: make(map[string]ledgerstate.Output),
		inputMap:  make(map[string]ledgerstate.Input),
		txMap:     make(map[string]*ledgerstate.Transaction),
	}
}

func (a *AliasManager) AddOutputAlias(output ledgerstate.Output, aliasName string) (err error) {
	if _, exists := a.outputMap[aliasName]; exists {
		err = errors.New("Duplicate alias name in output alias")
		return
	}

	a.outputMap[aliasName] = output
	return nil
}

func (a *AliasManager) AddInputAlias(input ledgerstate.Input, aliasName string) (err error) {
	if _, exists := a.inputMap[aliasName]; exists {
		err = errors.New("Duplicate alias name in input alias")
		return
	}

	a.inputMap[aliasName] = input
	return nil
}

func (a *AliasManager) AddTransactionAlias(tx *ledgerstate.Transaction, aliasName string) (err error) {
	if _, exists := a.txMap[aliasName]; exists {
		err = errors.New("Duplicate alias name in input alias")
		return
	}

	a.txMap[aliasName] = tx
	return nil
}

func (a *AliasManager) GetInput(aliasName string) ledgerstate.Input {
	return a.inputMap[aliasName]
}

func (a *AliasManager) GetOutput(aliasName string) ledgerstate.Output {
	return a.outputMap[aliasName]
}

func (a *AliasManager) ClearAliases() {
	a.inputMap = make(map[string]ledgerstate.Input)
	a.outputMap = make(map[string]ledgerstate.Output)
	a.txMap = make(map[string]*ledgerstate.Transaction)
}
