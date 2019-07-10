package validator

import (
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/model/bundle"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/bundleprocessor"
	"github.com/iotaledger/iota.go/address"
	. "github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/signing"
	. "github.com/iotaledger/iota.go/trinary"
)

var PLUGIN = node.NewPlugin("Validator", configure, run)

// Creates bundle signature fragments and the corresponding address to validate against.
// Each signature fragment after the first must go into its own meta transaction with value = 0.
func demoSign(seed Trytes, index uint64, sec SecurityLevel, bundleHash Hash) (Hash, []Trytes) {
	addr, _ := address.GenerateAddress(seed, index, sec)

	// compute seed based on address index
	subseed, _ := signing.Subseed(seed, index)
	// generate the private key
	prvKey, _ := signing.Key(subseed, sec)

	normalizedBundleHash := signing.NormalizedBundleHash(bundleHash)

	signatureFragments := make([]Trytes, sec)
	for i := 0; i < int(sec); i++ {
		// each security level signs one third of the (normalized) bundle hash
		signedFragTrits, _ := signing.SignatureFragment(
			normalizedBundleHash[i*HashTrytesSize/3:(i+1)*HashTrytesSize/3],
			prvKey[i*KeyFragmentLength:(i+1)*KeyFragmentLength],
		)
		signatureFragments[i] = MustTritsToTrytes(signedFragTrits)
	}

	return addr, signatureFragments
}

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
		valid, err := signing.ValidateSignatures(address, fragments, bundleHash, signing.NewKerl)
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
