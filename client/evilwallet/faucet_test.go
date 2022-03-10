package evilwallet

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestFaucetRequests(t *testing.T) {
	evilwallet := NewEvilWallet()

	wallet := evilwallet.NewWallet(fresh)
	clients := evilwallet.GetClients(2)

	_, err := evilwallet.RequestFreshFaucetWallet()
	require.NoError(t, err)

	err = evilwallet.RequestFreshBigFaucetWallet()
	require.NoError(t, err)

	evilwallet.RequestFreshBigFaucetWallets(5)

	txA, err := evilwallet.CreateTransaction("A", WithInputs("1"), WithOutput("2", 1000000), WithIssuer(wallet))
	require.NoError(t, err)

	_, err = clients[0].PostTransaction(txA.Bytes())
	require.NoError(t, err)

	evilwallet.ClearAliases()
	//EvilWallet.ConflictManager.AddConflict(WithConflictID("1"), WithConflictMembers("2", "3"))
}
