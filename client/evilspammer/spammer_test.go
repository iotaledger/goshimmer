package evilspammer

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/client/evilwallet"
	"github.com/stretchr/testify/require"
)

func TestSpamTransactions(t *testing.T) {
	evilWallet := evilwallet.NewEvilWallet()

	// request 100 outputs for spamming
	err := evilWallet.RequestFreshFaucetWallet()
	require.NoError(t, err)

	// simple transaction spam is the default scenario if none WithScenarioCustomConflicts option is provided in WithEvilScenario function.
	options := []Options{
		WithSpamRate(5, time.Second),
		WithBatchesSent(5),
		WithEvilWallet(evilWallet),
	}
	spammer := NewSpammer(options...)
	spammer.Spam()
}

func TestSpamDoubleSpend(t *testing.T) {
	evilWallet := evilwallet.NewEvilWallet()
	err := evilWallet.RequestFreshFaucetWallet()
	require.NoError(t, err)

	scenarioDs := evilwallet.NewEvilScenario(
		evilwallet.WithScenarioCustomConflicts(evilwallet.DoubleSpendBatch(5)),
	)

	options := []Options{
		WithSpamRate(5, time.Second),
		WithSpamDuration(time.Second * 10),
		WithEvilWallet(evilWallet),
	}
	dsOptions := append(options, WithEvilScenario(scenarioDs))

	dsSpammer := NewSpammer(dsOptions...)
	dsSpammer.Spam()
}

func TestCustomConflictScenario(t *testing.T) {
	evilWallet := evilwallet.NewEvilWallet()
	// request 10000 outputs for spamming
	err := evilWallet.RequestFreshFaucetWallet()
	require.NoError(t, err)

	customScenario := evilwallet.NewEvilScenario(
		evilwallet.WithScenarioCustomConflicts(evilwallet.Scenario1()),
	)

	options := []Options{
		WithSpamRate(5, time.Second),
		WithSpamDuration(time.Second * 10),
		WithEvilWallet(evilWallet),
	}
	customOptions := append(options, WithEvilScenario(customScenario))
	customSpammer := NewSpammer(customOptions...)

	customSpammer.Spam()
}

func TestReuseRestrictedOutputs(t *testing.T) {
	evilWallet := evilwallet.NewEvilWallet()

	err := evilWallet.RequestFreshFaucetWallet()
	require.NoError(t, err)

	// outputs from tx spam will be saved here, this wallet can be later reused as an input wallet for deep spam
	restrictedOutWallet := evilWallet.NewWallet(evilwallet.RestrictedReuse)

	scenarioTx := evilwallet.NewEvilScenario(
		evilwallet.WithScenarioReuseOutputWallet(restrictedOutWallet),
	)

	customScenario := evilwallet.NewEvilScenario(
		evilwallet.WithScenarioDeepSpamEnabled(),
		evilwallet.WithScenarioInputWalletForDeepSpam(restrictedOutWallet),
		evilwallet.WithScenarioCustomConflicts(evilwallet.Scenario1()),
	)

	options := []Options{
		WithSpamRate(5, time.Second),
		WithBatchesSent(1000),
		WithEvilWallet(evilWallet),
	}
	txOptions := append(options, WithEvilScenario(scenarioTx))
	customOptions := append(options, WithEvilScenario(customScenario))

	txSpammer := NewSpammer(txOptions...)
	customDeepSpammer := NewSpammer(customOptions...)

	txSpammer.Spam()
	customDeepSpammer.Spam()
}

func TestReuseOutputsOnTheFly(t *testing.T) {
	evilWallet := evilwallet.NewEvilWallet()

	err := evilWallet.RequestFreshFaucetWallet()
	require.NoError(t, err)
	outWallet := evilWallet.NewWallet(evilwallet.Reuse)

	customScenario := evilwallet.NewEvilScenario(
		evilwallet.WithScenarioDeepSpamEnabled(),
		evilwallet.WithScenarioInputWalletForDeepSpam(outWallet),
		evilwallet.WithScenarioReuseOutputWallet(outWallet),
		evilwallet.WithScenarioCustomConflicts(evilwallet.Scenario1()),
	)

	options := []Options{
		WithSpamRate(1, time.Second),
		WithBatchesSent(200),
		WithEvilWallet(evilWallet),
	}
	customOptions := append(options, WithEvilScenario(customScenario))

	customDeepSpammer := NewSpammer(customOptions...)

	customDeepSpammer.Spam()
}
