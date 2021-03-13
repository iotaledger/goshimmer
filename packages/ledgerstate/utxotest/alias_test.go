package utxotest

import (
	"fmt"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/ledgerstate/utxodb"
	"github.com/iotaledger/goshimmer/packages/ledgerstate/utxoutil"
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
	err = txb.AddReminderOutputIfNeeded(addr, nil)
	require.NoError(t, err)
	tx, err := txb.BuildWithED25519(user)
	require.NoError(t, err)

	err = u.AddTransaction(tx)
	require.NoError(t, err)

	chained, err := utxoutil.GetSingleChainedOutput(tx.Essence())
	require.NoError(t, err)
	require.NotNil(t, chained)

	t.Logf("Chained output: %s", chained)
	t.Logf("newly created alias address: %s", chained.GetAliasAddress().Base58())

	sender, err := utxoutil.GetSingleSender(tx, txb.ConsumedOutputs())
	require.NoError(t, err)
	require.True(t, sender.Equals(addr))
}

const chainLength = 10

func TestChain1(t *testing.T) {
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

	// mint chain output
	txb := utxoutil.NewBuilder(outputs...)
	err = txb.AddNewChainMint(bals1, addrStateControl, nil)
	require.NoError(t, err)
	err = txb.AddReminderOutputIfNeeded(addr, nil)
	require.NoError(t, err)
	tx, err := txb.BuildWithED25519(user)
	require.NoError(t, err)

	err = u.AddTransaction(tx)
	require.NoError(t, err)

	sender, err := utxoutil.GetSingleSender(tx, txb.ConsumedOutputs())
	require.NoError(t, err)
	require.True(t, sender.Equals(addr))

	chained, err := utxoutil.GetSingleChainedOutput(tx.Essence())
	require.NoError(t, err)
	require.NotNil(t, chained)

	aliasAddress := chained.GetAliasAddress()
	t.Logf("newly created alias address: %s", aliasAddress.Base58())

	require.EqualValues(t, utxodb.RequestFundsAmount-100, int(u.BalanceIOTA(addr)))
	require.EqualValues(t, 100, u.BalanceIOTA(aliasAddress))
	require.EqualValues(t, 0, u.BalanceIOTA(addrStateControl))

	for i := 0; i < chainLength; i++ {
		outputs = u.GetAddressOutputs(aliasAddress)
		require.EqualValues(t, 1, len(outputs))

		txb = utxoutil.NewBuilder(outputs...)
		chained, err = txb.ConsumeChainInputToOutput(aliasAddress)
		require.NoError(t, err)
		err = txb.AddOutputAndSpend(chained)
		require.NoError(t, err)
		tx, err = txb.BuildWithED25519(userStateControl)
		require.NoError(t, err)

		err = u.AddTransaction(tx)
		require.NoError(t, err)

		sender, err := utxoutil.GetSingleSender(tx, txb.ConsumedOutputs())
		require.NoError(t, err)
		require.True(t, sender.Equals(aliasAddress))

		require.EqualValues(t, 100, u.BalanceIOTA(aliasAddress))
		require.EqualValues(t, 0, u.BalanceIOTA(addrStateControl))
	}
}

func TestChain3(t *testing.T) {
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

	// mint chain output
	txb := utxoutil.NewBuilder(outputs...)
	err = txb.AddNewChainMint(bals1, addrStateControl, nil)
	require.NoError(t, err)
	err = txb.AddReminderOutputIfNeeded(addr, nil)
	require.NoError(t, err)
	tx, err := txb.BuildWithED25519(user)
	require.NoError(t, err)

	err = u.AddTransaction(tx)
	require.NoError(t, err)

	sender, err := utxoutil.GetSingleSender(tx, txb.ConsumedOutputs())
	require.NoError(t, err)
	require.True(t, sender.Equals(addr))

	chained, err := utxoutil.GetSingleChainedOutput(tx.Essence())
	require.NoError(t, err)
	require.NotNil(t, chained)

	aliasAddress := chained.GetAliasAddress()
	t.Logf("newly created alias address: %s", aliasAddress.Base58())

	require.EqualValues(t, utxodb.RequestFundsAmount-100, int(u.BalanceIOTA(addr)))
	require.EqualValues(t, 100, u.BalanceIOTA(aliasAddress))
	require.EqualValues(t, 0, u.BalanceIOTA(addrStateControl))

	for i := 0; i < chainLength; i++ {

		// transfer 1 more iota to alias address
		outputs = u.GetAddressOutputs(addr)
		txb := utxoutil.NewBuilder(outputs...)
		err = txb.AddExtendedOutputSimple(aliasAddress, nil, map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: 1})
		require.NoError(t, err)
		err = txb.AddReminderOutputIfNeeded(addr, nil)
		require.NoError(t, err)
		tx, err := txb.BuildWithED25519(user)
		require.NoError(t, err)
		err = u.AddTransaction(tx)
		require.NoError(t, err)

		sender, err := utxoutil.GetSingleSender(tx, txb.ConsumedOutputs())
		require.NoError(t, err)
		require.True(t, sender.Equals(addr))

		// continue chain without consuming ExtendedOutputs
		outputs = u.GetAddressOutputs(aliasAddress)
		require.EqualValues(t, 1+i+1, len(outputs))
		require.EqualValues(t, 0, u.BalanceIOTA(addrStateControl))

		txb = utxoutil.NewBuilder(outputs...)
		chained, err = txb.ConsumeChainInputToOutput(aliasAddress)
		require.NoError(t, err)
		err = txb.AddOutputAndSpend(chained)
		require.NoError(t, err)
		tx, err = txb.BuildWithED25519(userStateControl)
		require.NoError(t, err)

		err = u.AddTransaction(tx)
		require.NoError(t, err)

		sender, err = utxoutil.GetSingleSender(tx, txb.ConsumedOutputs())
		require.NoError(t, err)
		require.True(t, sender.Equals(aliasAddress))

		require.EqualValues(t, 100+i+1, u.BalanceIOTA(aliasAddress))
		require.EqualValues(t, 0, u.BalanceIOTA(addrStateControl))
	}
}

func TestChainWithExtendedOutput(t *testing.T) {
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

	// mint chain output
	txb := utxoutil.NewBuilder(outputs...)
	err = txb.AddNewChainMint(bals1, addrStateControl, nil)
	require.NoError(t, err)
	err = txb.AddReminderOutputIfNeeded(addr, nil)
	require.NoError(t, err)
	tx, err := txb.BuildWithED25519(user)
	require.NoError(t, err)

	err = u.AddTransaction(tx)
	require.NoError(t, err)

	sender, err := utxoutil.GetSingleSender(tx, txb.ConsumedOutputs())
	require.NoError(t, err)
	require.True(t, sender.Equals(addr))

	chained, err := utxoutil.GetSingleChainedOutput(tx.Essence())
	require.NoError(t, err)
	require.NotNil(t, chained)

	aliasAddress := chained.GetAliasAddress()
	t.Logf("newly created alias address: %s", aliasAddress.Base58())

	require.EqualValues(t, utxodb.RequestFundsAmount-100, int(u.BalanceIOTA(addr)))
	require.EqualValues(t, 100, u.BalanceIOTA(aliasAddress))
	require.EqualValues(t, 0, u.BalanceIOTA(addrStateControl))

	for i := 0; i < chainLength; i++ {
		// transfer 1 more iota to alias address
		outputs = u.GetAddressOutputs(addr)
		txb = utxoutil.NewBuilder(outputs...)
		err = txb.AddExtendedOutputSimple(aliasAddress, nil, map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: 1})
		require.NoError(t, err)
		err = txb.AddReminderOutputIfNeeded(addr, nil)
		require.NoError(t, err)
		tx, err := txb.BuildWithED25519(user)
		require.NoError(t, err)
		err = u.AddTransaction(tx)
		require.NoError(t, err)

		sender, err := utxoutil.GetSingleSender(tx, txb.ConsumedOutputs())
		require.NoError(t, err)
		require.True(t, sender.Equals(addr))

		// continue chain with consuming ExtendedOutput
		outputs = u.GetAddressOutputs(aliasAddress)
		require.EqualValues(t, 2, len(outputs))
		require.EqualValues(t, 0, u.BalanceIOTA(addrStateControl))

		txb = utxoutil.NewBuilder(outputs...)
		chained, err = txb.ConsumeChainInputToOutput(aliasAddress)
		require.NoError(t, err)
		txb.ConsumeReminderBalances(true)
		err = chained.SetBalances(txb.ConsumedUnspent())
		require.NoError(t, err)
		err = txb.AddOutputAndSpend(chained)
		require.NoError(t, err)
		tx, err = txb.BuildWithED25519(userStateControl)
		require.NoError(t, err)

		err = u.AddTransaction(tx)
		require.NoError(t, err)

		sender, err = utxoutil.GetSingleSender(tx, txb.ConsumedOutputs())
		require.NoError(t, err)
		require.True(t, sender.Equals(aliasAddress))

		require.EqualValues(t, 100+i+1, u.BalanceIOTA(aliasAddress))
		require.EqualValues(t, 0, u.BalanceIOTA(addrStateControl))
	}
}

func TestRequestSendingPattern(t *testing.T) {
	u := utxodb.New()
	userRequester := utxodb.NewKeyPairFromSeed(2)
	addrRequester := ledgerstate.NewED25519Address(userRequester.PublicKey)
	_, err := u.RequestFunds(addrRequester)
	require.NoError(t, err)
	require.EqualValues(t, utxodb.Supply-utxodb.RequestFundsAmount, u.BalanceIOTA(u.GetGenesisAddress()))
	require.EqualValues(t, utxodb.RequestFundsAmount, u.BalanceIOTA(addrRequester))

	// start chain with 100 iotas on it
	userStateControl := utxodb.NewKeyPairFromSeed(3)
	addrStateControl := ledgerstate.NewED25519Address(userStateControl.PublicKey)
	bals1 := map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: 100}
	require.NoError(t, err)

	outputs := u.GetAddressOutputs(addrRequester)
	require.EqualValues(t, 1, len(outputs))

	// mint chain output
	txb := utxoutil.NewBuilder(outputs...)
	err = txb.AddNewChainMint(bals1, addrStateControl, nil)
	require.NoError(t, err)
	err = txb.AddReminderOutputIfNeeded(addrRequester, nil)
	require.NoError(t, err)
	tx, err := txb.BuildWithED25519(userRequester)
	require.NoError(t, err)

	err = u.AddTransaction(tx)
	require.NoError(t, err)

	sender, err := utxoutil.GetSingleSender(tx, txb.ConsumedOutputs())
	require.NoError(t, err)
	require.True(t, sender.Equals(addrRequester))

	chained, err := utxoutil.GetSingleChainedOutput(tx.Essence())
	require.NoError(t, err)
	require.NotNil(t, chained)

	aliasAddress := chained.GetAliasAddress()
	t.Logf("newly created alias address: %s", aliasAddress.Base58())

	require.EqualValues(t, utxodb.RequestFundsAmount-100, int(u.BalanceIOTA(addrRequester)))
	require.EqualValues(t, 100, u.BalanceIOTA(aliasAddress))
	require.EqualValues(t, 0, u.BalanceIOTA(addrStateControl))

	const numRequests = 10
	for i := 0; i < numRequests; i++ {
		// send request with 1 iota and some data to alias address
		outputs = u.GetAddressOutputs(addrRequester)
		txb = utxoutil.NewBuilder(outputs...)
		data := []byte(fmt.Sprintf("#%d", i))
		err = txb.AddExtendedOutputSimple(aliasAddress, data, map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: 1})
		require.NoError(t, err)
		err = txb.AddReminderOutputIfNeeded(addrRequester, nil)
		require.NoError(t, err)
		tx, err := txb.BuildWithED25519(userRequester)
		require.NoError(t, err)
		err = u.AddTransaction(tx)
		require.NoError(t, err)

		sender, err := utxoutil.GetSingleSender(tx, txb.ConsumedOutputs())
		require.NoError(t, err)
		require.True(t, sender.Equals(addrRequester))
	}
	// continue chain with consuming ExtendedOutput
	outputs = u.GetAddressOutputs(aliasAddress)
	require.EqualValues(t, 1+numRequests, len(outputs))
	require.EqualValues(t, 0, u.BalanceIOTA(addrStateControl))
	require.EqualValues(t, 100+numRequests, int(u.BalanceIOTA(aliasAddress)))

	txb = utxoutil.NewBuilder(outputs...)
	chained, err = txb.ConsumeChainInputToOutput(aliasAddress)
	require.NoError(t, err)
	txb.ForEachUntouched(func(o ledgerstate.Output, idx int) bool {
		_ = txb.ConsumeUntouchedByIndex(idx)
		return true
	})
	txb.ConsumeReminderBalances(false)
	err = chained.SetBalances(txb.ConsumedUnspent())
	require.NoError(t, err)
	err = txb.AddOutputAndSpend(chained)
	require.NoError(t, err)
	tx, err = txb.BuildWithED25519(userStateControl)
	require.NoError(t, err)
	//
	err = u.AddTransaction(tx)
	require.NoError(t, err)

	sender, err = utxoutil.GetSingleSender(tx, txb.ConsumedOutputs())
	require.NoError(t, err)
	require.True(t, sender.Equals(aliasAddress))

	require.EqualValues(t, 100+numRequests, int(u.BalanceIOTA(aliasAddress)))
	require.EqualValues(t, 0, u.BalanceIOTA(addrStateControl))

	outputs = u.GetAddressOutputs(aliasAddress)
	require.EqualValues(t, 1, len(outputs))
}
