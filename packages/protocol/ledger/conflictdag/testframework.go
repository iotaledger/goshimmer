package conflictdag

import (
	"fmt"
	"testing"

	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

type TestFramework struct {
	t *testing.T

	Instance *ConflictDAG[utxo.TransactionID, utxo.OutputID]

	conflictIDsByAlias map[string]utxo.TransactionID
	resourceByAlias    map[string]utxo.OutputID
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework(t *testing.T, conflictDAGInstance *ConflictDAG[utxo.TransactionID, utxo.OutputID]) *TestFramework {
	return &TestFramework{
		t:                  t,
		Instance:           conflictDAGInstance,
		conflictIDsByAlias: make(map[string]utxo.TransactionID),
		resourceByAlias:    make(map[string]utxo.OutputID),
	}
}

func (t *TestFramework) randomConflictID() (randomConflictID utxo.TransactionID) {
	if err := randomConflictID.FromRandomness(); err != nil {
		panic(err)
	}

	return randomConflictID
}

func (t *TestFramework) randomResourceID() (randomConflictID utxo.OutputID) {
	if err := randomConflictID.FromRandomness(); err != nil {
		panic(err)
	}

	return randomConflictID
}

func (t *TestFramework) CreateConflict(conflictSetAlias, conflictAlias string, parentConflictIDs utxo.TransactionIDs) {
	if _, exists := t.resourceByAlias[conflictSetAlias]; !exists {
		t.resourceByAlias[conflictSetAlias] = t.randomResourceID()
		t.resourceByAlias[conflictSetAlias].RegisterAlias(conflictSetAlias)
	}

	t.conflictIDsByAlias[conflictAlias] = t.randomConflictID()
	t.conflictIDsByAlias[conflictAlias].RegisterAlias(conflictAlias)

	t.Instance.CreateConflict(t.conflictIDsByAlias[conflictAlias], parentConflictIDs, t.ConflictSetIDs(conflictSetAlias))
}

func (t *TestFramework) ConflictID(alias string) (conflictID utxo.TransactionID) {
	conflictID, ok := t.conflictIDsByAlias[alias]
	if !ok {
		panic(fmt.Sprintf("ConflictID alias %s not registered", alias))
	}

	return
}

func (t *TestFramework) ConflictIDs(aliases ...string) (conflictIDs utxo.TransactionIDs) {
	conflictIDs = utxo.NewTransactionIDs()
	for _, alias := range aliases {
		conflictIDs.Add(t.ConflictID(alias))
	}

	return
}

func (t *TestFramework) ConflictSetID(alias string) (conflictSetID utxo.OutputID) {
	conflictSetID, ok := t.resourceByAlias[alias]
	if !ok {
		panic(fmt.Sprintf("ConflictSetID alias %s not registered", alias))
	}

	return
}

func (t *TestFramework) ConflictSetIDs(aliases ...string) (conflictSetIDs utxo.OutputIDs) {
	conflictSetIDs = utxo.NewOutputIDs()
	for _, alias := range aliases {
		conflictSetIDs.Add(t.ConflictSetID(alias))
	}

	return
}
