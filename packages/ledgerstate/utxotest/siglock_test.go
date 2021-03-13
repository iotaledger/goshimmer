package utxotest

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate/utxodb"
	"testing"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/ledgerstate/utxoutil"
	"github.com/stretchr/testify/require"
)

func TestSendIotas(t *testing.T) {
	u := utxodb.New()
	user1 := utxodb.NewKeyPairFromSeed(1)
	addr1 := ledgerstate.NewED25519Address(user1.PublicKey)
	_, err := u.RequestFunds(addr1)
	require.NoError(t, err)

	user2 := utxodb.NewKeyPairFromSeed(2)
	addr2 := ledgerstate.NewED25519Address(user2.PublicKey)

	require.EqualValues(t, utxodb.RequestFundsAmount, u.BalanceIOTA(addr1))
	require.EqualValues(t, 0, u.BalanceIOTA(addr2))

	outputs := u.GetAddressOutputs(addr1)
	require.EqualValues(t, 1, len(outputs))
	txb := utxoutil.NewBuilder(outputs...)
	err = txb.AddSigLockedIOTAOutput(addr2, 42)
	require.NoError(t, err)
	err = txb.AddReminderOutputIfNeeded(addr1, nil)
	require.NoError(t, err)

	tx, err := txb.BuildWithED25519(user1)
	require.NoError(t, err)
	err = u.AddTransaction(tx)
	require.NoError(t, err)

	require.EqualValues(t, utxodb.RequestFundsAmount-42, u.BalanceIOTA(addr1))
	require.EqualValues(t, 42, u.BalanceIOTA(addr2))
}

const howMany = uint64(42)

func TestSendIotasMany(t *testing.T) {
	u := utxodb.New()
	user1 := utxodb.NewKeyPairFromSeed(1)
	addr1 := ledgerstate.NewED25519Address(user1.PublicKey)
	_, err := u.RequestFunds(addr1)
	require.NoError(t, err)

	user2 := utxodb.NewKeyPairFromSeed(2)
	addr2 := ledgerstate.NewED25519Address(user2.PublicKey)
	require.EqualValues(t, 0, u.BalanceIOTA(addr2))

	require.EqualValues(t, utxodb.RequestFundsAmount, u.BalanceIOTA(addr1))
	require.EqualValues(t, 0, u.BalanceIOTA(addr2))

	for i := uint64(0); i < howMany; i++ {
		outputs := u.GetAddressOutputs(addr1)
		require.EqualValues(t, 1, len(outputs))
		txb := utxoutil.NewBuilder(outputs...)

		err := txb.AddSigLockedIOTAOutput(addr2, 1)
		require.NoError(t, err)
		err = txb.AddReminderOutputIfNeeded(addr1, nil)
		require.NoError(t, err)

		tx, err := txb.BuildWithED25519(user1)
		require.NoError(t, err)

		err = u.AddTransaction(tx)
		require.NoError(t, err)

		sender, err := utxoutil.GetSingleSender(tx, txb.ConsumedOutputs())
		require.NoError(t, err)
		require.True(t, addr1.Equals(sender))
	}
	require.EqualValues(t, utxodb.RequestFundsAmount-howMany, u.BalanceIOTA(addr1))
	require.EqualValues(t, howMany, u.BalanceIOTA(addr2))
}

func TestSendIotas1FromMany(t *testing.T) {
	u := utxodb.New()
	user1 := utxodb.NewKeyPairFromSeed(1)
	addr1 := ledgerstate.NewED25519Address(user1.PublicKey)
	_, err := u.RequestFunds(addr1)
	require.NoError(t, err)

	user2 := utxodb.NewKeyPairFromSeed(2)
	addr2 := ledgerstate.NewED25519Address(user2.PublicKey)
	require.EqualValues(t, 0, u.BalanceIOTA(addr2))

	require.EqualValues(t, utxodb.RequestFundsAmount, u.BalanceIOTA(addr1))
	require.EqualValues(t, 0, u.BalanceIOTA(addr2))

	const outputAmount = 5

	for i := uint64(0); i < howMany; i++ {
		outputs := u.GetAddressOutputs(addr1)
		require.EqualValues(t, 1, len(outputs))

		txb := utxoutil.NewBuilder(outputs...)
		err := txb.AddSigLockedIOTAOutput(addr2, outputAmount)
		require.NoError(t, err)
		err = txb.AddReminderOutputIfNeeded(addr1, nil)
		require.NoError(t, err)

		tx, err := txb.BuildWithED25519(user1)
		require.NoError(t, err)
		err = u.AddTransaction(tx)
		require.NoError(t, err)

		sender, err := utxoutil.GetSingleSender(tx, txb.ConsumedOutputs())
		require.NoError(t, err)
		require.True(t, sender.Equals(addr1))
	}

	require.EqualValues(t, utxodb.RequestFundsAmount-howMany*outputAmount, u.BalanceIOTA(addr1))
	require.EqualValues(t, howMany*outputAmount, u.BalanceIOTA(addr2))

	// transfer 1 back
	outputs := u.GetAddressOutputs(addr2)
	require.EqualValues(t, howMany, len(outputs))

	txb := utxoutil.NewBuilder(outputs...)
	err = txb.AddSigLockedIOTAOutput(addr1, outputAmount)
	require.NoError(t, err)
	err = txb.AddReminderOutputIfNeeded(addr2, nil)
	require.NoError(t, err)
	tx, err := txb.BuildWithED25519(user2)
	require.NoError(t, err)
	err = u.AddTransaction(tx)
	require.NoError(t, err)

	sender, err := utxoutil.GetSingleSender(tx, txb.ConsumedOutputs())
	require.NoError(t, err)
	require.True(t, sender.Equals(addr2))

	require.EqualValues(t, utxodb.RequestFundsAmount-howMany*outputAmount+outputAmount, u.BalanceIOTA(addr1))
	require.EqualValues(t, howMany*outputAmount-outputAmount, u.BalanceIOTA(addr2))

	// transfer 3 more back
	outputs = u.GetAddressOutputs(addr2)
	require.EqualValues(t, howMany-1, len(outputs))

	txb = utxoutil.NewBuilder(outputs...)
	err = txb.AddSigLockedIOTAOutput(addr1, 3)
	require.NoError(t, err)
	err = txb.AddReminderOutputIfNeeded(addr2, nil)
	require.NoError(t, err)
	tx, err = txb.BuildWithED25519(user2)
	require.NoError(t, err)
	err = u.AddTransaction(tx)
	require.NoError(t, err)

	sender, err = utxoutil.GetSingleSender(tx, txb.ConsumedOutputs())
	require.NoError(t, err)
	require.True(t, sender.Equals(addr2))

	require.EqualValues(t, utxodb.RequestFundsAmount-howMany*outputAmount+outputAmount+3, u.BalanceIOTA(addr1))
	require.EqualValues(t, howMany*outputAmount-outputAmount-3, u.BalanceIOTA(addr2))
}

func TestSendIotasManyFromMany(t *testing.T) {
	u := utxodb.New()
	user1 := utxodb.NewKeyPairFromSeed(1)
	addr1 := ledgerstate.NewED25519Address(user1.PublicKey)
	_, err := u.RequestFunds(addr1)
	require.NoError(t, err)

	user2 := utxodb.NewKeyPairFromSeed(2)
	addr2 := ledgerstate.NewED25519Address(user2.PublicKey)
	require.EqualValues(t, 0, u.BalanceIOTA(addr2))

	require.EqualValues(t, utxodb.RequestFundsAmount, u.BalanceIOTA(addr1))
	require.EqualValues(t, 0, u.BalanceIOTA(addr2))

	const outputAmount = 5

	for i := uint64(0); i < howMany; i++ {
		outputs := u.GetAddressOutputs(addr1)
		require.EqualValues(t, 1, len(outputs))
		txb := utxoutil.NewBuilder(outputs...)
		err := txb.AddSigLockedIOTAOutput(addr2, outputAmount)
		require.NoError(t, err)
		err = txb.AddReminderOutputIfNeeded(addr1, nil)
		require.NoError(t, err)
		tx, err := txb.BuildWithED25519(user1)
		require.NoError(t, err)
		err = u.AddTransaction(tx)
		require.NoError(t, err)

		sender, err := utxoutil.GetSingleSender(tx, txb.ConsumedOutputs())
		require.NoError(t, err)
		require.True(t, sender.Equals(addr1))
	}

	require.EqualValues(t, utxodb.RequestFundsAmount-howMany*outputAmount, u.BalanceIOTA(addr1))
	require.EqualValues(t, howMany*outputAmount, u.BalanceIOTA(addr2))

	outputs := u.GetAddressOutputs(addr2)
	require.EqualValues(t, howMany, len(outputs))

	txb := utxoutil.NewBuilder(outputs...)
	err = txb.AddSigLockedIOTAOutput(addr1, howMany*outputAmount/2)
	require.NoError(t, err)
	tx, err := txb.BuildWithED25519(user2)
	require.NoError(t, err)
	err = u.AddTransaction(tx)
	require.NoError(t, err)

	sender, err := utxoutil.GetSingleSender(tx, txb.ConsumedOutputs())
	require.NoError(t, err)
	require.True(t, sender.Equals(addr2))

	require.EqualValues(t, utxodb.RequestFundsAmount-howMany*outputAmount/2, u.BalanceIOTA(addr1))
	require.EqualValues(t, howMany*outputAmount/2, u.BalanceIOTA(addr2))

	outputs = u.GetAddressOutputs(addr2)
	require.EqualValues(t, int(howMany/2), len(outputs))
}
