package evilwallet

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeepDoubleSpend(t *testing.T) {
	evilwallet := NewEvilWallet()

	err, wallet := evilwallet.RequestFundsFromFaucet(WithOutputAlias("1"))
	require.NoError(t, err)

	err = evilwallet.SendCustomConflicts([]ConflictSlice{
		{
			// split funds
			[]Option{WithInputs("1"), WithOutputs([]*OutputOption{{aliasName: "2"}, {aliasName: "3"}}), WithIssuer(wallet)},
		},
		{
			[]Option{WithInputs("2"), WithOutput(&OutputOption{aliasName: "4", amount: 500000})},
			[]Option{WithInputs("2"), WithOutput(&OutputOption{aliasName: "5", amount: 500000})},
		},
		{
			[]Option{WithInputs("3"), WithOutput(&OutputOption{aliasName: "6", amount: 500000})},
			[]Option{WithInputs("3"), WithOutput(&OutputOption{aliasName: "7", amount: 500000})},
		},
		{
			// aggregated
			[]Option{WithInputs([]string{"5", "6"}), WithOutput(&OutputOption{aliasName: "8", amount: 1000000})},
		},
	})
	require.NoError(t, err)
	evilwallet.ClearAllAliases()
}
