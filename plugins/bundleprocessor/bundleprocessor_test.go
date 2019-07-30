package bundleprocessor

import (
	"sync"
	"testing"

	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/model/bundle"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"

	"github.com/iotaledger/goshimmer/plugins/tipselection"

	"github.com/iotaledger/goshimmer/packages/client"

	"github.com/iotaledger/goshimmer/packages/node"

	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/iota.go/consts"
	"github.com/magiconair/properties/assert"
)

var seed = client.NewSeed("YFHQWAUPCXC9S9DSHP9NDF9RLNPMZVCMSJKUKQP9SWUSUCPRQXCMDVDVZ9SHHESHIQNCXWBJF9UJSWE9Z", consts.SecurityLevelMedium)

func BenchmarkValidateSignatures(b *testing.B) {
	bundleFactory := client.NewBundleFactory()
	bundleFactory.AddInput(seed.GetAddress(0), -400)
	bundleFactory.AddOutput(seed.GetAddress(1), 400, "Testmessage")
	bundleFactory.AddOutput(client.NewAddress("SJKUKQP9SWUSUCPRQXCMDVDVZ9SHHESHIQNCXWBJF9UJSWE9ZYFHQWAUPCXC9S9DSHP9NDF9RLNPMZVCM"), 400, "Testmessage")

	generatedBundle := bundleFactory.GenerateBundle(tipselection.GetRandomTip(), tipselection.GetRandomTip())

	b.ResetTimer()

	var wg sync.WaitGroup

	for i := 0; i < b.N; i++ {
		wg.Add(1)

		go func() {
			ValidateSignatures(generatedBundle.GetEssenceHash(), generatedBundle.GetTransactions())

			wg.Done()
		}()
	}

	wg.Wait()
}

func TestValidateSignatures(t *testing.T) {
	bundleFactory := client.NewBundleFactory()
	bundleFactory.AddInput(seed.GetAddress(0), -400)
	bundleFactory.AddOutput(seed.GetAddress(1), 400, "Testmessage")
	bundleFactory.AddOutput(client.NewAddress("SJKUKQP9SWUSUCPRQXCMDVDVZ9SHHESHIQNCXWBJF9UJSWE9ZYFHQWAUPCXC9S9DSHP9NDF9RLNPMZVCM"), 400, "Testmessage")

	generatedBundle := bundleFactory.GenerateBundle(tipselection.GetRandomTip(), tipselection.GetRandomTip())

	successful, err := ValidateSignatures(generatedBundle.GetEssenceHash(), generatedBundle.GetTransactions())
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, successful, true, "validation failed")
}

func TestProcessSolidBundleHead_Data(t *testing.T) {
	// show all error messages for tests
	*node.LOG_LEVEL.Value = node.LOG_LEVEL_FAILURE

	// start a test node
	node.Start(tangle.PLUGIN, PLUGIN)

	bundleFactory := client.NewBundleFactory()
	bundleFactory.AddOutput(seed.GetAddress(1), 400, "Testmessage")
	bundleFactory.AddOutput(client.NewAddress("SJKUKQP9SWUSUCPRQXCMDVDVZ9SHHESHIQNCXWBJF9UJSWE9ZYFHQWAUPCXC9S9DSHP9NDF9RLNPMZVCM"), 400, "Testmessage")

	generatedBundle := bundleFactory.GenerateBundle(tipselection.GetRandomTip(), tipselection.GetRandomTip())

	for _, transaction := range generatedBundle.GetTransactions() {
		tangle.StoreTransaction(transaction)
	}

	var wg sync.WaitGroup

	testResults := events.NewClosure(func(bundle *bundle.Bundle, transactions []*value_transaction.ValueTransaction) {
		assert.Equal(t, bundle.GetHash(), generatedBundle.GetTransactions()[0].GetHash(), "invalid bundle hash")
		assert.Equal(t, bundle.IsValueBundle(), false, "invalid value bundle status")

		wg.Done()
	})
	Events.BundleSolid.Attach(testResults)

	wg.Add(1)

	if err := ProcessSolidBundleHead(generatedBundle.GetTransactions()[0]); err != nil {
		t.Error(err)
	}

	wg.Wait()

	Events.BundleSolid.Detach(testResults)

	// shutdown test node
	node.Shutdown()
}

func TestProcessSolidBundleHead_Value(t *testing.T) {
	// show all error messages for tests
	*node.LOG_LEVEL.Value = node.LOG_LEVEL_FAILURE

	// start a test node
	node.Start(tangle.PLUGIN, PLUGIN)

	bundleFactory := client.NewBundleFactory()
	bundleFactory.AddInput(seed.GetAddress(0), -400)
	bundleFactory.AddOutput(seed.GetAddress(1), 400, "Testmessage")
	bundleFactory.AddOutput(client.NewAddress("SJKUKQP9SWUSUCPRQXCMDVDVZ9SHHESHIQNCXWBJF9UJSWE9ZYFHQWAUPCXC9S9DSHP9NDF9RLNPMZVCM"), 400, "Testmessage")

	generatedBundle := bundleFactory.GenerateBundle(tipselection.GetRandomTip(), tipselection.GetRandomTip())

	for _, transaction := range generatedBundle.GetTransactions() {
		tangle.StoreTransaction(transaction)
	}

	var wg sync.WaitGroup

	testResults := events.NewClosure(func(bundle *bundle.Bundle, transactions []*value_transaction.ValueTransaction) {
		assert.Equal(t, bundle.GetHash(), generatedBundle.GetTransactions()[0].GetHash(), "invalid bundle hash")
		assert.Equal(t, bundle.GetBundleEssenceHash(), generatedBundle.GetEssenceHash(), "invalid bundle essence hash")
		assert.Equal(t, bundle.IsValueBundle(), true, "invalid value bundle status")

		wg.Done()
	})

	Events.BundleSolid.Attach(testResults)

	wg.Add(1)

	if err := ProcessSolidBundleHead(generatedBundle.GetTransactions()[0]); err != nil {
		t.Error(err)
	}

	wg.Wait()

	Events.BundleSolid.Detach(testResults)

	// shutdown test node
	node.Shutdown()
}
