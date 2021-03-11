package utxotest

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/ledgerstate/utxotest/utxodb"
	"github.com/iotaledger/goshimmer/packages/ledgerstate/utxotest/utxoutil"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAliasMint(t *testing.T) {
	u := utxodb.New()
	user := utxodb.NewKeyPairFromSeed(2)
	addr := ledgerstate.NewED25519Address(user.PublicKey)
	_, err := u.RequestFunds(addr)
	require.NoError(t, err)
	require.EqualValues(t, utxodb.Supply-utxodb.RequestFundsAmount, u.BalanceIOTA(u.GetGenesisAddress()))
	require.EqualValues(t, utxodb.RequestFundsAmount, u.BalanceIOTA(addr))

	userStateControl := utxodb.NewKeyPairFromSeed(3)
	addrStateControl := ledgerstate.NewED25519Address(userStateControl.PublicKey)
	bals1 := map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: 100}
	require.NoError(t, err)

	outputs := u.GetAddressOutputs(addr)
	require.EqualValues(t, 1, len(outputs))

	txb := utxoutil.NewBuilder(outputs...)
	err = txb.AddNewChainMint(bals1, addrStateControl, nil)
	require.NoError(t, err)
	err = txb.AddReminderOutputIfNeeded(addr)
	require.NoError(t, err)
	tx, err := txb.BuildWithED25519(user)
	require.NoError(t, err)

	err = u.AddTransaction(tx)
	require.NoError(t, err)

	chained, err := utxoutil.GetSingleChainedOutput(tx.Essence())
	require.NoError(t, err)

	t.Logf("Chained output: %s", chained)
	t.Logf("newly created alias address: %s", chained.GetAliasAddress().Base58())
}

func TestChain(t *testing.T) {
	u := utxodb.New()
	user := utxodb.NewKeyPairFromSeed(2)
	addr := ledgerstate.NewED25519Address(user.PublicKey)
	_, err := u.RequestFunds(addr)
	require.NoError(t, err)
	require.EqualValues(t, utxodb.Supply-utxodb.RequestFundsAmount, u.BalanceIOTA(u.GetGenesisAddress()))
	require.EqualValues(t, utxodb.RequestFundsAmount, u.BalanceIOTA(addr))

	userStateControl := utxodb.NewKeyPairFromSeed(3)
	addrStateControl := ledgerstate.NewED25519Address(userStateControl.PublicKey)
	bals1 := map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: 100}
	require.NoError(t, err)

	outputs := u.GetAddressOutputs(addr)
	require.EqualValues(t, 1, len(outputs))

	txb := utxoutil.NewBuilder(outputs...)
	err = txb.AddNewChainMint(bals1, addrStateControl, nil)
	require.NoError(t, err)
	err = txb.AddReminderOutputIfNeeded(addr)
	require.NoError(t, err)
	tx, err := txb.BuildWithED25519(user)
	require.NoError(t, err)

	err = u.AddTransaction(tx)
	require.NoError(t, err)

	chained, err := utxoutil.GetSingleChainedOutput(tx.Essence())
	require.NoError(t, err)

	aliasAddress := chained.GetAliasAddress()
	t.Logf("newly created alias address: %s", aliasAddress.Base58())

	outputs = u.GetAddressOutputs(aliasAddress)
	require.EqualValues(t, 1, len(outputs))

	txb = utxoutil.NewBuilder(outputs...)
	chained, err = txb.ConsumeChainInput(aliasAddress)
	require.NoError(t, err)
	txb.SpendConsumedUnspent()
	err = txb.AddOutput(chained)
	require.NoError(t, err)
	tx, err = txb.BuildWithED25519(userStateControl)
	require.NoError(t, err)

	err = u.AddTransaction(tx)
	require.NoError(t, err)
}
