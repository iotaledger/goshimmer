package utxodb

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestBasic(t *testing.T) {
	u := New()
	genTx, ok := u.GetTransaction(u.genesisTxId)
	require.True(t, ok)
	require.EqualValues(t, genTx.ID(), u.genesisTxId)
}

func TestGenesis(t *testing.T) {
	u := New()
	require.EqualValues(t, u.Supply(), u.BalanceIOTA(u.GetGenesisAddress()))
	u.checkLedgerBalance()
}

func TestRequestFunds(t *testing.T) {
	u := New()
	_, addr := u.NewKeyPairByIndex(2)
	_, err := u.RequestFunds(addr)
	require.NoError(t, err)
	require.EqualValues(t, u.Supply()-RequestFundsAmount, u.BalanceIOTA(u.GetGenesisAddress()))
	require.EqualValues(t, RequestFundsAmount, u.BalanceIOTA(addr))
	u.checkLedgerBalance()
}

func TestAddTransactionFail(t *testing.T) {
	u := New()
	_, addr := u.NewKeyPairByIndex(2)
	tx, err := u.RequestFunds(addr)
	require.NoError(t, err)
	require.EqualValues(t, u.Supply()-RequestFundsAmount, u.BalanceIOTA(u.GetGenesisAddress()))
	require.EqualValues(t, RequestFundsAmount, u.BalanceIOTA(addr))
	u.checkLedgerBalance()
	err = u.AddTransaction(tx)
	require.Error(t, err)
	u.checkLedgerBalance()
}
