package utxotest

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate/utxotest/utxodb"
	"testing"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/ledgerstate/utxotest/utxoutil"
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
	err = txb.AddReminderOutput(addr1)
	require.NoError(t, err)

	tx, err := txb.BuildWithED25519(user1)
	require.NoError(t, err)
	err = u.AddTransaction(tx)
	require.NoError(t, err)

	require.EqualValues(t, utxodb.RequestFundsAmount-42, u.BalanceIOTA(addr1))
	require.EqualValues(t, 42, u.BalanceIOTA(addr2))
}

//
//const howMany = uint64(42)
//
//func TestSendIotasMany(t *testing.T) {
//	u := utxodb.New()
//	user1 := utxodb.NewKeyPairFromSeed(1)
//	addr1 := ledgerstate.NewED25519Address(user1.PublicKey)
//	_, err := u.RequestFunds(addr1)
//	require.NoError(t, err)
//
//	user2 := utxodb.NewKeyPairFromSeed(2)
//	addr2 := ledgerstate.NewED25519Address(user2.PublicKey)
//	require.EqualValues(t, 0, u.BalanceIOTA(addr2))
//
//	require.EqualValues(t, utxodb.RequestFundsAmount, u.BalanceIOTA(addr1))
//	require.EqualValues(t, 0, u.BalanceIOTA(addr2))
//
//	for i := uint64(0); i < howMany; i++ {
//		outputs := u.GetAddressOutputs(addr1)
//		require.EqualValues(t, 1, len(outputs))
//		txb := utxoutil.NewBuilder(outputs)
//		idx, err := txb.AddSigLockedIOTAOutput(addr2, 1)
//		require.NoError(t, err)
//		require.EqualValues(t, 0, idx)
//		tx, err := txb.BuildWithED25519(user1)
//		require.NoError(t, err)
//		err = u.AddTransaction(tx)
//		require.NoError(t, err)
//	}
//	require.EqualValues(t, utxodb.RequestFundsAmount-howMany, u.BalanceIOTA(addr1))
//	require.EqualValues(t, howMany, u.BalanceIOTA(addr2))
//}
//
//func TestSendIotas1FromMany(t *testing.T) {
//	u := utxodb.New()
//	user1 := utxodb.NewKeyPairFromSeed(1)
//	addr1 := ledgerstate.NewED25519Address(user1.PublicKey)
//	_, err := u.RequestFunds(addr1)
//	require.NoError(t, err)
//
//	user2 := utxodb.NewKeyPairFromSeed(2)
//	addr2 := ledgerstate.NewED25519Address(user2.PublicKey)
//	require.EqualValues(t, 0, u.BalanceIOTA(addr2))
//
//	require.EqualValues(t, utxodb.RequestFundsAmount, u.BalanceIOTA(addr1))
//	require.EqualValues(t, 0, u.BalanceIOTA(addr2))
//
//	for i := uint64(0); i < howMany; i++ {
//		outputs := u.GetAddressOutputs(addr1)
//		require.EqualValues(t, 1, len(outputs))
//		txb := utxoutil.NewBuilder(ledgerstate.NewOutputs(outputs...))
//		idx, err := txb.AddSigLockedIOTAOutput(addr2, 1)
//		require.NoError(t, err)
//		require.EqualValues(t, 0, idx)
//		tx, err := txb.BuildWithED25519(user1)
//		require.NoError(t, err)
//		err = u.AddTransaction(tx)
//		require.NoError(t, err)
//	}
//
//	require.EqualValues(t, utxodb.RequestFundsAmount-howMany, u.BalanceIOTA(addr1))
//	require.EqualValues(t, howMany, u.BalanceIOTA(addr2))
//
//	outputs := u.GetAddressOutputs(addr2)
//	require.EqualValues(t, howMany, len(outputs))
//
//	txb := utxoutil.NewBuilder(outputs)
//	idx, err := txb.AddSigLockedIOTAOutput(addr1, 1)
//	require.NoError(t, err)
//	require.EqualValues(t, 0, idx)
//	tx, err := txb.BuildWithED25519(user2)
//	require.NoError(t, err)
//	err = u.AddTransaction(tx)
//	require.NoError(t, err)
//
//	require.EqualValues(t, utxodb.RequestFundsAmount-howMany+1, u.BalanceIOTA(addr1))
//	require.EqualValues(t, howMany-1, u.BalanceIOTA(addr2))
//}
//
//func TestSendIotasManyFromMany(t *testing.T) {
//	u := utxodb.New()
//	user1 := utxodb.NewKeyPairFromSeed(1)
//	addr1 := ledgerstate.NewED25519Address(user1.PublicKey)
//	_, err := u.RequestFunds(addr1)
//	require.NoError(t, err)
//
//	user2 := utxodb.NewKeyPairFromSeed(2)
//	addr2 := ledgerstate.NewED25519Address(user2.PublicKey)
//	require.EqualValues(t, 0, u.BalanceIOTA(addr2))
//
//	require.EqualValues(t, utxodb.RequestFundsAmount, u.BalanceIOTA(addr1))
//	require.EqualValues(t, 0, u.BalanceIOTA(addr2))
//
//	for i := uint64(0); i < howMany; i++ {
//		outputs := u.GetAddressOutputs(addr1)
//		require.EqualValues(t, 1, len(outputs))
//		txb := utxoutil.NewBuilder(ledgerstate.NewOutputs(outputs...))
//		idx, err := txb.AddSigLockedIOTAOutput(addr2, 1)
//		require.NoError(t, err)
//		require.EqualValues(t, 0, idx)
//		tx, err := txb.BuildWithED25519(user1)
//		require.NoError(t, err)
//		err = u.AddTransaction(tx)
//		require.NoError(t, err)
//	}
//
//	require.EqualValues(t, utxodb.RequestFundsAmount-howMany, u.BalanceIOTA(addr1))
//	require.EqualValues(t, howMany, u.BalanceIOTA(addr2))
//
//	outputs := u.GetAddressOutputs(addr2)
//	require.EqualValues(t, howMany, len(outputs))
//
//	txb := utxoutil.NewBuilder(outputs)
//	idx, err := txb.AddSigLockedIOTAOutput(addr1, howMany/2)
//	require.NoError(t, err)
//	require.EqualValues(t, 0, idx)
//	tx, err := txb.BuildWithED25519(user2)
//	require.NoError(t, err)
//	err = u.AddTransaction(tx)
//	require.NoError(t, err)
//
//	require.EqualValues(t, utxodb.RequestFundsAmount-howMany/2, u.BalanceIOTA(addr1))
//	require.EqualValues(t, howMany/2, u.BalanceIOTA(addr2))
//
//	outputs = u.GetAddressOutputs(addr2)
//	require.EqualValues(t, int(howMany/2), len(outputs))
//}
