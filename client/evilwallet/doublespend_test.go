package evilwallet

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDoubleSpend(t *testing.T) {
	evilwallet := NewEvilWallet()

	clients := evilwallet.GetClients(2)

	err := evilwallet.RequestFundsFromFaucet(WithOutputAlias("1"))
	require.NoError(t, err)

	txA, err := evilwallet.CreateTransaction("A", WithInputs("1"), WithOutput("2", 1000000))
	require.NoError(t, err)

	txB, err := evilwallet.CreateTransaction("B", WithInputs("1"), WithOutput("3", 1000000))
	require.NoError(t, err)

	res1, err := clients[0].PostTransaction(txA.Bytes())
	require.NoError(t, err)

	res2, err := clients[1].PostTransaction(txB.Bytes())
	require.NoError(t, err)

	evilwallet.ClearAliases()
	//EvilWallet.ConflictManager.AddConflict(WithConflictID("1"), WithConflictMembers("2", "3"))

	// assert the conflict has been created
	metadata1, err := clients[0].GetTransactionMetadata(res1.TransactionID)
	require.NoError(t, err)
	metadata2, err := clients[1].GetTransactionMetadata(res2.TransactionID)
	require.NoError(t, err)
	require.NotEqual(t, metadata1.BranchIDs, metadata2.BranchIDs)
}
