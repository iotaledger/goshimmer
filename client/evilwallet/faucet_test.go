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
		txA, err := evilwallet.CreateTransaction(WithInputs("1"), WithOutput(&OutputOption{aliasName: "2"}))
		require.NoError(t, err)
		txB, err := evilwallet.CreateTransaction(WithInputs("1"), WithOutput(&OutputOption{aliasName: "3"}))
		require.NoError(t, err)
		_, err = clients[0].PostTransaction(txA.Bytes())
		require.NoError(t, err)
		_, err = clients[1].PostTransaction(txB.Bytes())
		require.NoError(t, err)
		evilwallet.ClearAliases()
	}

	err = evilwallet.RequestFreshBigFaucetWallet()
	require.NoError(t, err)

	evilwallet.RequestFreshBigFaucetWallets(5)

}
