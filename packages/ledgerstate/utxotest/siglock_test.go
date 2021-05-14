package utxotest

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/ledgerstate/utxodb"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/ledgerstate/utxoutil"
)

func TestSendIotas(t *testing.T) {
	u := utxodb.New()
	user1, addr1 := u.NewKeyPairByIndex(1)
	_, err := u.RequestFunds(addr1)
	require.NoError(t, err)

	_, addr2 := u.NewKeyPairByIndex(2)

	require.EqualValues(t, utxodb.RequestFundsAmount, u.BalanceIOTA(addr1))
	require.EqualValues(t, 0, u.BalanceIOTA(addr2))

	outputs := u.GetAddressOutputs(addr1)
	require.EqualValues(t, 1, len(outputs))
	txb := utxoutil.NewBuilder(outputs...)
	err = txb.AddSigLockedIOTAOutput(addr2, 42)
	require.NoError(t, err)
	err = txb.AddRemainderOutputIfNeeded(addr1, nil)
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
	user1, addr1 := u.NewKeyPairByIndex(1)
	_, err := u.RequestFunds(addr1)
	require.NoError(t, err)

	_, addr2 := u.NewKeyPairByIndex(2)
	require.EqualValues(t, 0, u.BalanceIOTA(addr2))

	require.EqualValues(t, utxodb.RequestFundsAmount, u.BalanceIOTA(addr1))
	require.EqualValues(t, 0, u.BalanceIOTA(addr2))

	for i := uint64(0); i < howMany; i++ {
		outputs := u.GetAddressOutputs(addr1)
		require.EqualValues(t, 1, len(outputs))
		txb := utxoutil.NewBuilder(outputs...)

		err = txb.AddSigLockedIOTAOutput(addr2, 1)
		require.NoError(t, err)
		err = txb.AddRemainderOutputIfNeeded(addr1, nil)
		require.NoError(t, err)

		tx, err1 := txb.BuildWithED25519(user1)
		require.NoError(t, err1)

		err = u.AddTransaction(tx)
		require.NoError(t, err)

		sender, err2 := utxoutil.GetSingleSender(tx)
		require.NoError(t, err2)
		require.True(t, addr1.Equals(sender))
	}
	require.EqualValues(t, utxodb.RequestFundsAmount-howMany, u.BalanceIOTA(addr1))
	require.EqualValues(t, howMany, u.BalanceIOTA(addr2))
}

func TestSendIotas1FromMany(t *testing.T) {
	u := utxodb.New()
	user1, addr1 := u.NewKeyPairByIndex(1)
	_, err := u.RequestFunds(addr1)
	require.NoError(t, err)

	user2, addr2 := u.NewKeyPairByIndex(2)
	require.EqualValues(t, 0, u.BalanceIOTA(addr2))

	require.EqualValues(t, utxodb.RequestFundsAmount, u.BalanceIOTA(addr1))
	require.EqualValues(t, 0, u.BalanceIOTA(addr2))

	const outputAmount = 5

	for i := uint64(0); i < howMany; i++ {
		outputs := u.GetAddressOutputs(addr1)
		require.EqualValues(t, 1, len(outputs))

		txb := utxoutil.NewBuilder(outputs...)
		err = txb.AddSigLockedIOTAOutput(addr2, outputAmount)
		require.NoError(t, err)
		err = txb.AddRemainderOutputIfNeeded(addr1, nil)
		require.NoError(t, err)

		tx, bErr := txb.BuildWithED25519(user1)
		require.NoError(t, bErr)
		err = u.AddTransaction(tx)
		require.NoError(t, err)

		sender, sErr := utxoutil.GetSingleSender(tx)
		require.NoError(t, sErr)
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
	err = txb.AddRemainderOutputIfNeeded(addr2, nil)
	require.NoError(t, err)
	tx, err := txb.BuildWithED25519(user2)
	require.NoError(t, err)
	err = u.AddTransaction(tx)
	require.NoError(t, err)

	sender, err := utxoutil.GetSingleSender(tx)
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
	err = txb.AddRemainderOutputIfNeeded(addr2, nil)
	require.NoError(t, err)
	tx, err = txb.BuildWithED25519(user2)
	require.NoError(t, err)
	err = u.AddTransaction(tx)
	require.NoError(t, err)

	sender, err = utxoutil.GetSingleSender(tx)
	require.NoError(t, err)
	require.True(t, sender.Equals(addr2))

	require.EqualValues(t, utxodb.RequestFundsAmount-howMany*outputAmount+outputAmount+3, u.BalanceIOTA(addr1))
	require.EqualValues(t, howMany*outputAmount-outputAmount-3, u.BalanceIOTA(addr2))
}

func TestSendIotasManyFromMany(t *testing.T) {
	u := utxodb.New()
	user1, addr1 := u.NewKeyPairByIndex(1)
	_, err := u.RequestFunds(addr1)
	require.NoError(t, err)

	user2, addr2 := u.NewKeyPairByIndex(2)
	require.EqualValues(t, 0, u.BalanceIOTA(addr2))

	require.EqualValues(t, utxodb.RequestFundsAmount, u.BalanceIOTA(addr1))
	require.EqualValues(t, 0, u.BalanceIOTA(addr2))

	const outputAmount = 5

	for i := uint64(0); i < howMany; i++ {
		outputs := u.GetAddressOutputs(addr1)
		require.EqualValues(t, 1, len(outputs))
		txb := utxoutil.NewBuilder(outputs...)
		err = txb.AddSigLockedIOTAOutput(addr2, outputAmount)
		require.NoError(t, err)
		err = txb.AddRemainderOutputIfNeeded(addr1, nil)
		require.NoError(t, err)
		tx, err1 := txb.BuildWithED25519(user1)
		require.NoError(t, err1)
		err = u.AddTransaction(tx)
		require.NoError(t, err)

		sender, err2 := utxoutil.GetSingleSender(tx)
		require.NoError(t, err2)
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

	sender, err := utxoutil.GetSingleSender(tx)
	require.NoError(t, err)
	require.True(t, sender.Equals(addr2))

	require.EqualValues(t, utxodb.RequestFundsAmount-howMany*outputAmount/2, u.BalanceIOTA(addr1))
	require.EqualValues(t, howMany*outputAmount/2, u.BalanceIOTA(addr2))

	outputs = u.GetAddressOutputs(addr2)
	require.EqualValues(t, int(howMany/2), len(outputs))
}
