package evilwallet

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDoubleSpend(t *testing.T) {
	evilwallet := NewEvilWallet()

	wallet := evilwallet.NewWallet(custom)
	clients := evilwallet.GetClients(2)

	err := evilwallet.RequestFundsFromFaucet(wallet, WithOutputAlias("1"))
	require.NoError(t, err)

	txA, err := evilwallet.CreateTransaction(WithInputs("1"), WithOutput("2", 1000000), WithIssuer(wallet))
	require.NoError(t, err)

	txB, err := evilwallet.CreateTransaction(WithInputs("1"), WithOutput("3", 1000000), WithIssuer(wallet))
	require.NoError(t, err)

	_, err = clients[0].PostTransaction(txA.Bytes())
	require.NoError(t, err)

	_, err = clients[1].PostTransaction(txB.Bytes())
	require.NoError(t, err)

	evilwallet.ClearAliases()
	//EvilWallet.ConflictManager.AddConflict(WithConflictID("1"), WithConflictMembers("2", "3"))
}
