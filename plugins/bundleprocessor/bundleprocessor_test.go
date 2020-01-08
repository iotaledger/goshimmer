package bundleprocessor

import (
	"sync"
	"testing"

	"github.com/iotaledger/goshimmer/packages/client"
	"github.com/iotaledger/goshimmer/packages/model/bundle"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/parameter"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/goshimmer/plugins/tipselection"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/iota.go/consts"
	"github.com/magiconair/properties/assert"
	"github.com/spf13/viper"
)

var seed = client.NewSeed("YFHQWAUPCXC9S9DSHP9NDF9RLNPMZVCMSJKUKQP9SWUSUCPRQXCMDVDVZ9SHHESHIQNCXWBJF9UJSWE9Z", consts.SecurityLevelMedium)

func init() {
	err := parameter.LoadDefaultConfig(false)
	if err != nil {
		log.Fatalf("Failed to initialize config: %s", err)
	}
	logger.InitGlobalLogger(&viper.Viper{})
}

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

func TestProcessSolidBundleHead(t *testing.T) {
	// start a test node
	node.Start(node.Plugins(tangle.PLUGIN, PLUGIN))
	defer node.Shutdown()

	t.Run("data", func(t *testing.T) {
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
		defer Events.BundleSolid.Detach(testResults)

		wg.Add(1)
		if err := ProcessSolidBundleHead(generatedBundle.GetTransactions()[0]); err != nil {
			t.Error(err)
		}

		wg.Wait()
	})

	t.Run("value", func(t *testing.T) {
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
			assert.Equal(t, bundle.IsValueBundle(), true, "invalid value bundle status")

			wg.Done()
		})

		wg.Add(1)
		Events.BundleSolid.Attach(testResults)
		defer Events.BundleSolid.Detach(testResults)

		if err := ProcessSolidBundleHead(generatedBundle.GetTransactions()[0]); err != nil {
			t.Error(err)
		}

		wg.Wait()
	})
}
