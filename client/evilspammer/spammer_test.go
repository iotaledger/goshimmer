package evilspammer

import (
	"github.com/iotaledger/goshimmer/client/evilwallet"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestSpamTransactions(t *testing.T) {
	evilWallet := evilwallet.NewEvilWallet()

	err := evilWallet.RequestFreshBigFaucetWallet()
	require.NoError(t, err)

	outWallet := evilWallet.NewWallet(evilwallet.Reuse)

	scenario := evilwallet.NewEvilScenario(evilwallet.WithScenarioReuseOutputWallet(outWallet))
	options := []Options{
		WithSpamRate(5, time.Second),
		WithBatchesSent(20),
		WithSpammingFunc(ValueSpammingFunc),
		WithSpamWallet(evilWallet),
		WithEvilScenario(scenario),
	}
	spammer := NewSpammer(options...)
	spammer.Spam()
}

func TestSpamDoubleSpend(t *testing.T) {
	evilWallet := evilwallet.NewEvilWallet()

	err := evilWallet.RequestFreshBigFaucetWallet()
	require.NoError(t, err)

	scenarioTx := evilwallet.NewEvilScenario()
	scenarioDs := evilwallet.NewEvilScenario(evilwallet.WithScenarioCustomConflicts(evilwallet.DoubleSpendBatch(5)))
	customScenario := evilwallet.NewEvilScenario(evilwallet.WithScenarioCustomConflicts(evilwallet.Scenario1()))

	options := []Options{
		WithSpamRate(5, time.Second),
		WithSpamDuration(time.Second * 10),
		WithSpammingFunc(CustomConflictSpammingFunc),
		WithSpamWallet(evilWallet),
	}
	txOptions := append(options, WithEvilScenario(scenarioTx))
	dsOptions := append(options, WithEvilScenario(scenarioDs))
	customOptions := append(options, WithEvilScenario(customScenario))

	txSpammer := NewSpammer(txOptions...)
	dsSpammer := NewSpammer(dsOptions...)
	customSpammer := NewSpammer(customOptions...)

	txSpammer.Spam()
	dsSpammer.Spam()
	customSpammer.Spam()

}

func TestReuseOutputs(t *testing.T) {
	evilWallet := evilwallet.NewEvilWallet()

	err := evilWallet.RequestFreshBigFaucetWallet()
	require.NoError(t, err)

	outWallet := evilWallet.NewWallet(evilwallet.Reuse)
	restrictedOutWallet := evilWallet.NewWallet(evilwallet.RestrictedReuse)

	scenarioTx := evilwallet.NewEvilScenario(evilwallet.WithScenarioReuseOutputWallet(restrictedOutWallet))

	scenarioDeepTx := evilwallet.NewEvilScenario(
		evilwallet.WithScenarioReuseOutputWallet(outWallet),
		evilwallet.WithScenarioDeepSpamEnabled(),
	)

	customScenario := evilwallet.NewEvilScenario(
		evilwallet.WithScenarioDeepSpamEnabled(),
		evilwallet.WithScenarioInputWalletForDeepSpam(restrictedOutWallet),
		evilwallet.WithScenarioCustomConflicts(evilwallet.Scenario1()),
	)

	options := []Options{
		WithSpamRate(5, time.Second),
		WithSpamDuration(time.Second * 10),
		WithSpammingFunc(CustomConflictSpammingFunc),
		WithSpamWallet(evilWallet),
	}
	txOptions := append(options, WithEvilScenario(scenarioTx))
	deepTxOptions := append(options, WithEvilScenario(scenarioDeepTx))
	customOptions := append(options, WithEvilScenario(customScenario))

	txSpammer := NewSpammer(txOptions...)
	txDeepSpammer := NewSpammer(deepTxOptions...)
	customDeepSpammer := NewSpammer(customOptions...)

	txSpammer.Spam()
	txDeepSpammer.Spam()
	customDeepSpammer.Spam()
}
