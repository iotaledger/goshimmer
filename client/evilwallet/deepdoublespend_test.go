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
			"A": []Option{WithInputs("1"), WithOutputs([]string{"2", "3"}), WithIssuer(wallet)},
		},
		{
			"B": []Option{WithInputs("2"), WithOutput("4", 500000), WithIssuer(wallet)},
			"C": []Option{WithInputs("2"), WithOutput("5", 500000), WithIssuer(wallet)},
		},
		{
			"D": []Option{WithInputs("3"), WithOutput("6", 500000), WithIssuer(wallet)},
			"E": []Option{WithInputs("3"), WithOutput("7", 500000), WithIssuer(wallet)},
		},
		{
			// aggregated
			"F": []Option{WithInputs("5", "6"), WithOutput("8", 1000000), WithIssuer(wallet)},
		},
	}, evilwallet.GetClients(2))

	require.NoError(t, err)
}
