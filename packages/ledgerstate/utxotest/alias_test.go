package utxotest

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/ledgerstate/utxotest/utxodb"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAliasMint(t *testing.T) {
	u := utxodb.New()
	user := utxodb.NewKeyPairFromSeed(2)
	addr := ledgerstate.NewED25519Address(user.PublicKey)
	_, err := u.RequestFunds(addr)
	require.NoError(t, err)
	require.EqualValues(t, utxodb.Supply-utxodb.RequestFundsAmount, u.BalanceIOTA(u.GetGenesisAddress()))
	require.EqualValues(t, utxodb.RequestFundsAmount, u.BalanceIOTA(addr))

	userStateControl := utxodb.NewKeyPairFromSeed(3)
	addrStateControl := ledgerstate.NewED25519Address(userStateControl.PublicKey)
	bals1 := map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: 100}
	_, err = ledgerstate.NewChainOutputMint(bals1, addrStateControl)
	require.NoError(t, err)

	outputs := u.GetAddressOutputs(addr)
	require.EqualValues(t, 1, len(outputs))

	//txb := utxoutil.NewBuilder(outputs)
	//err = txb.addOutput(aliasMint)
	//require.NoError(t, err)
	//
	//_, err = txb.BuildWithED25519(user)
	//require.NoError(t, err)
}
