package evilwallet

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeepDoubleSpend(t *testing.T) {
	evilwallet := NewEvilWallet()

	wallet := evilwallet.NewWallet(fresh)

	err := evilwallet.RequestFundsFromFaucet(wallet, WithOutputAlias("1"))
	require.NoError(t, err)

	err = evilwallet.SendCustomConflicts([]ConflictMap{
		{
			// split funds
			[]Option{WithInputs("1"), WithOutputs([]string{"2", "3"}), WithIssuer(wallet)},
		},
		{
			[]Option{WithInputs("2"), WithOutput("4", 500000), WithIssuer(wallet)},
			[]Option{WithInputs("2"), WithOutput("5", 500000), WithIssuer(wallet)},
		},
		{
			[]Option{WithInputs("3"), WithOutput("6", 500000), WithIssuer(wallet)},
			[]Option{WithInputs("3"), WithOutput("7", 500000), WithIssuer(wallet)},
		},
		{
			// aggregated
			[]Option{WithInputs("5", "6"), WithOutput("8", 1000000), WithIssuer(wallet)},
		},
	}, evilwallet.GetClients(2))

	require.NoError(t, err)
}
