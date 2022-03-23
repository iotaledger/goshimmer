package evilwallet

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDoubleSpend(t *testing.T) {
	evilwallet := NewEvilWallet()

	clients := evilwallet.GetClients(2)

	err, initWallet := evilwallet.RequestFundsFromFaucet(WithOutputAlias("1"))
	require.NoError(t, err)

	txA, err := evilwallet.CreateTransaction(WithInputs("1"), WithOutput(&OutputOption{aliasName: "2", amount: 1000000}), WithIssuer(initWallet))
	require.NoError(t, err)

	txB, err := evilwallet.CreateTransaction(WithInputs("1"), WithOutput(&OutputOption{aliasName: "3", amount: 1000000}), WithIssuer(initWallet))
	require.NoError(t, err)

	_, err = clients[0].PostTransaction(txA)
	require.NoError(t, err)

	_, err = clients[1].PostTransaction(txB)
	require.NoError(t, err)

	evilwallet.ClearAliases()
	//EvilWallet.ConflictManager.AddConflict(WithConflictID("1"), WithConflictMembers("2", "3"))
}
