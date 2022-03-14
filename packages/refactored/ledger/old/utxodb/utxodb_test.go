package utxodb

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"

	"github.com/stretchr/testify/require"
)

func TestBasic(t *testing.T) {
	u := New()
	genTx, ok := u.GetTransaction(u.genesisTxID)
	require.True(t, ok)
	require.EqualValues(t, genTx.ID(), u.genesisTxID)
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

func TestGetOutput(t *testing.T) {
	u := New()
	originOutputID := ledgerstate.NewOutputID(u.GenesisTransactionID(), 0)

	succ := u.GetOutputMetadata(originOutputID, func(metadata *ledgerstate.OutputMetadata) {
		require.EqualValues(t, 0, metadata.ConsumerCount())
	})
	require.True(t, succ)

	_, addr := u.NewKeyPairByIndex(2)
	tx, err := u.RequestFunds(addr)
	require.NoError(t, err)
	require.EqualValues(t, u.Supply()-RequestFundsAmount, u.BalanceIOTA(u.GetGenesisAddress()))
	require.EqualValues(t, RequestFundsAmount, u.BalanceIOTA(addr))
	u.checkLedgerBalance()

	succ = u.GetOutputMetadata(originOutputID, func(metadata *ledgerstate.OutputMetadata) {
		require.EqualValues(t, 1, metadata.ConsumerCount())
	})
	require.True(t, succ)

	outid0 := ledgerstate.NewOutputID(tx.ID(), 0)
	outid1 := ledgerstate.NewOutputID(tx.ID(), 1)
	outidFail := ledgerstate.NewOutputID(tx.ID(), 5)

	var out0, out1 ledgerstate.Output
	succ = u.GetOutput(outid0, func(output ledgerstate.Output) {
		out0 = output
	})
	require.True(t, succ)
	bal, ok := out0.Balances().Get(ledgerstate.ColorIOTA)
	require.True(t, ok)
	require.EqualValues(t, RequestFundsAmount, bal)

	succ = u.GetOutput(outid1, func(output ledgerstate.Output) {
		out1 = output
	})
	require.True(t, succ)
	bal, ok = out1.Balances().Get(ledgerstate.ColorIOTA)
	require.True(t, ok)
	require.EqualValues(t, u.Supply()-RequestFundsAmount, bal)

	require.NotPanics(t, func() {
		succ = u.GetOutput(outidFail, nil)
	})
	require.False(t, succ)

	succ = u.GetOutputMetadata(outid0, func(metadata *ledgerstate.OutputMetadata) {
		require.True(t, metadata.Solid())
		require.Equal(t, 0, metadata.ConsumerCount())
	})
	require.True(t, succ)
}
