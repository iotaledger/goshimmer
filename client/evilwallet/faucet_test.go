package evilwallet

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFaucetRequests(t *testing.T) {
	evilwallet := NewEvilWallet()

	clients := evilwallet.GetClients(2)

	_, err := evilwallet.RequestFreshFaucetWallet()
	require.NoError(t, err)

	_, err = evilwallet.RequestFreshFaucetWallet()
	require.NoError(t, err)

	for i := 0; i < 200; i++ {
		txA, err := evilwallet.CreateTransaction("A", WithInputs("1"), WithOutputs([]string{"2"}))
		require.NoError(t, err)
		txB, err := evilwallet.CreateTransaction("B", WithInputs("1"), WithOutputs([]string{"3"}))
		require.NoError(t, err)
		_, err = clients[0].PostTransaction(txA)
		require.NoError(t, err)
		_, err = clients[1].PostTransaction(txB)
		require.NoError(t, err)

		evilwallet.ClearAliases()
	}

	err = evilwallet.RequestFreshBigFaucetWallet()
	require.NoError(t, err)

	evilwallet.RequestFreshBigFaucetWallets(5)

}
