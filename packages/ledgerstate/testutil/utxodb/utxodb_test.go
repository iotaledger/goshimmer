package utxodb

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/ledgerstate/testutil/txutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasic(t *testing.T) {
	u := New()
	genTx, ok := u.GetTransaction(u.genesisTxId)
	assert.Equal(t, ok, true)
	assert.Equal(t, genTx.ID(), u.genesisTxId)
}

func TestGenesis(t *testing.T) {
	u := New()
	require.EqualValues(t, supply, u.BalanceIOTA(u.GetGenesisAddress()))
	u.checkLedgerBalance()
}

func TestRequestFunds(t *testing.T) {
	u := New()
	user := NewKeyPairFromSeed(2)
	addr := ledgerstate.NewED25519Address(user.PublicKey)
	_, err := u.RequestFunds(addr)
	require.NoError(t, err)
	require.EqualValues(t, supply-RequestFundsAmount, u.BalanceIOTA(u.GetGenesisAddress()))
	require.EqualValues(t, RequestFundsAmount, u.BalanceIOTA(addr))
	u.checkLedgerBalance()
}

func TestAddTransactionFail(t *testing.T) {
	u := New()
	user := NewKeyPairFromSeed(2)
	addr := ledgerstate.NewED25519Address(user.PublicKey)
	tx, err := u.RequestFunds(addr)
	require.NoError(t, err)
	require.EqualValues(t, supply-RequestFundsAmount, u.BalanceIOTA(u.GetGenesisAddress()))
	require.EqualValues(t, RequestFundsAmount, u.BalanceIOTA(addr))
	u.checkLedgerBalance()
	err = u.AddTransaction(tx)
	require.Error(t, err)
	u.checkLedgerBalance()
}

func TestSendIotas(t *testing.T) {
	u := New()
	user1 := NewKeyPairFromSeed(1)
	addr1 := ledgerstate.NewED25519Address(user1.PublicKey)
	_, err := u.RequestFunds(addr1)
	require.NoError(t, err)

	user2 := NewKeyPairFromSeed(2)
	addr2 := ledgerstate.NewED25519Address(user2.PublicKey)

	require.EqualValues(t, RequestFundsAmount, u.BalanceIOTA(addr1))
	require.EqualValues(t, 0, u.BalanceIOTA(addr2))

	outputs := u.GetAddressOutputs(addr1)
	require.EqualValues(t, 1, len(outputs))
	txb := txutil.NewBuilder(ledgerstate.NewOutputs(outputs...))
	idx, err := txb.AddIOTAOutput(addr2, 42)
	require.EqualValues(t, 0, idx)
	require.NoError(t, err)
	tx, err := txb.BuildWithED25519(user1)
	require.NoError(t, err)
	err = u.AddTransaction(tx)
	require.NoError(t, err)

	require.EqualValues(t, RequestFundsAmount-42, u.BalanceIOTA(addr1))
	require.EqualValues(t, 42, u.BalanceIOTA(addr2))
}

const howMany = uint64(42)

func TestSendIotasMany(t *testing.T) {
	u := New()
	user1 := NewKeyPairFromSeed(1)
	addr1 := ledgerstate.NewED25519Address(user1.PublicKey)
	_, err := u.RequestFunds(addr1)
	require.NoError(t, err)

	user2 := NewKeyPairFromSeed(2)
	addr2 := ledgerstate.NewED25519Address(user2.PublicKey)
	require.EqualValues(t, 0, u.BalanceIOTA(addr2))

	require.EqualValues(t, RequestFundsAmount, u.BalanceIOTA(addr1))
	require.EqualValues(t, 0, u.BalanceIOTA(addr2))

	for i := uint64(0); i < howMany; i++ {
		outputs := u.GetAddressOutputs(addr1)
		require.EqualValues(t, 1, len(outputs))
		txb := txutil.NewBuilder(ledgerstate.NewOutputs(outputs...))
		idx, err := txb.AddIOTAOutput(addr2, 1)
		require.NoError(t, err)
		require.EqualValues(t, 0, idx)
		tx, err := txb.BuildWithED25519(user1)
		require.NoError(t, err)
		err = u.AddTransaction(tx)
		require.NoError(t, err)
	}
	require.EqualValues(t, RequestFundsAmount-howMany, u.BalanceIOTA(addr1))
	require.EqualValues(t, howMany, u.BalanceIOTA(addr2))
}

func TestSendIotas1FromMany(t *testing.T) {
	u := New()
	user1 := NewKeyPairFromSeed(1)
	addr1 := ledgerstate.NewED25519Address(user1.PublicKey)
	_, err := u.RequestFunds(addr1)
	require.NoError(t, err)

	user2 := NewKeyPairFromSeed(2)
	addr2 := ledgerstate.NewED25519Address(user2.PublicKey)
	require.EqualValues(t, 0, u.BalanceIOTA(addr2))

	require.EqualValues(t, RequestFundsAmount, u.BalanceIOTA(addr1))
	require.EqualValues(t, 0, u.BalanceIOTA(addr2))

	for i := uint64(0); i < howMany; i++ {
		outputs := u.GetAddressOutputs(addr1)
		require.EqualValues(t, 1, len(outputs))
		txb := txutil.NewBuilder(ledgerstate.NewOutputs(outputs...))
		idx, err := txb.AddIOTAOutput(addr2, 1)
		require.NoError(t, err)
		require.EqualValues(t, 0, idx)
		tx, err := txb.BuildWithED25519(user1)
		require.NoError(t, err)
		err = u.AddTransaction(tx)
		require.NoError(t, err)
	}

	require.EqualValues(t, RequestFundsAmount-howMany, u.BalanceIOTA(addr1))
	require.EqualValues(t, howMany, u.BalanceIOTA(addr2))

	outputs := u.GetAddressOutputs(addr2)
	require.EqualValues(t, howMany, len(outputs))

	txb := txutil.NewBuilder(outputs)
	idx, err := txb.AddIOTAOutput(addr1, 1)
	require.NoError(t, err)
	require.EqualValues(t, 0, idx)
	tx, err := txb.BuildWithED25519(user2)
	require.NoError(t, err)
	err = u.AddTransaction(tx)
	require.NoError(t, err)

	require.EqualValues(t, RequestFundsAmount-howMany+1, u.BalanceIOTA(addr1))
	require.EqualValues(t, howMany-1, u.BalanceIOTA(addr2))
}

func TestSendIotasManyFromMany(t *testing.T) {
	u := New()
	user1 := NewKeyPairFromSeed(1)
	addr1 := ledgerstate.NewED25519Address(user1.PublicKey)
	_, err := u.RequestFunds(addr1)
	require.NoError(t, err)

	user2 := NewKeyPairFromSeed(2)
	addr2 := ledgerstate.NewED25519Address(user2.PublicKey)
	require.EqualValues(t, 0, u.BalanceIOTA(addr2))

	require.EqualValues(t, RequestFundsAmount, u.BalanceIOTA(addr1))
	require.EqualValues(t, 0, u.BalanceIOTA(addr2))

	for i := uint64(0); i < howMany; i++ {
		outputs := u.GetAddressOutputs(addr1)
		require.EqualValues(t, 1, len(outputs))
		txb := txutil.NewBuilder(ledgerstate.NewOutputs(outputs...))
		idx, err := txb.AddIOTAOutput(addr2, 1)
		require.NoError(t, err)
		require.EqualValues(t, 0, idx)
		tx, err := txb.BuildWithED25519(user1)
		require.NoError(t, err)
		err = u.AddTransaction(tx)
		require.NoError(t, err)
	}

	require.EqualValues(t, RequestFundsAmount-howMany, u.BalanceIOTA(addr1))
	require.EqualValues(t, howMany, u.BalanceIOTA(addr2))

	outputs := u.GetAddressOutputs(addr2)
	require.EqualValues(t, howMany, len(outputs))

	txb := txutil.NewBuilder(outputs)
	idx, err := txb.AddIOTAOutput(addr1, howMany/2)
	require.NoError(t, err)
	require.EqualValues(t, 0, idx)
	tx, err := txb.BuildWithED25519(user2)
	require.NoError(t, err)
	err = u.AddTransaction(tx)
	require.NoError(t, err)

	require.EqualValues(t, RequestFundsAmount-howMany/2, u.BalanceIOTA(addr1))
	require.EqualValues(t, howMany/2, u.BalanceIOTA(addr2))

	outputs = u.GetAddressOutputs(addr2)
	require.EqualValues(t, int(howMany/2), len(outputs))
}
