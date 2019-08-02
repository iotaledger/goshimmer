package validator

import (
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/model/bundle"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/bundleprocessor"
	"github.com/iotaledger/iota.go/kerl"
	"github.com/iotaledger/iota.go/signing"
	. "github.com/iotaledger/iota.go/trinary"
)

var PLUGIN = node.NewPlugin("Validator", node.Enabled, configure, run)

func validateSignatures(bundleHash Hash, txs []*value_transaction.ValueTransaction) (bool, error) {
	for i, tx := range txs {
		// ignore all non-input transactions
		if tx.GetValue() >= 0 {
			continue
		}

		address := tx.GetAddress()

		// it is unknown how many fragments there will be
		fragments := []Trytes{tx.GetSignatureMessageFragment()}

		// each consecutive meta transaction with the same address contains another signature fragment
		for j := i; j < len(txs)-1; j++ {
			otherTx := txs[j+1]
			if otherTx.GetValue() != 0 || otherTx.GetAddress() != address {
				break
			}

			fragments = append(fragments, otherTx.GetSignatureMessageFragment())
		}

		// validate all the fragments against the address using Kerl
		valid, err := signing.ValidateSignatures(address, fragments, bundleHash, kerl.NewKerl())
		if err != nil {
			return false, err
		}
		if !valid {
			return false, nil
		}
	}

	return true, nil
}

func configure(plugin *node.Plugin) {

	bundleprocessor.Events.BundleSolid.Attach(events.NewClosure(func(b *bundle.Bundle, txs []*value_transaction.ValueTransaction) {
		// signature are verified against the bundle hash
		valid, _ := validateSignatures(b.GetBundleEssenceHash(), txs)
		if !valid {
			plugin.LogFailure("Invalid signature")
		}
	}))
}

func run(*node.Plugin) {
}
