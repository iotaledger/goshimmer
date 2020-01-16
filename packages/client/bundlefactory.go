package client

import (
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/converter"
	"github.com/iotaledger/iota.go/signing"
	"github.com/iotaledger/iota.go/trinary"
)

type BundleFactory struct {
	inputs  []bundleFactoryInputEntry
	outputs []bundleFactoryOutputEntry
}

func NewBundleFactory() *BundleFactory {
	return &BundleFactory{
		inputs:  make([]bundleFactoryInputEntry, 0),
		outputs: make([]bundleFactoryOutputEntry, 0),
	}
}

func (bundleFactory *BundleFactory) AddInput(address *Address, value int64) {
	bundleFactory.inputs = append(bundleFactory.inputs, bundleFactoryInputEntry{
		address: address,
		value:   value,
	})
}

func (bundleFactory *BundleFactory) AddOutput(address *Address, value int64, message ...string) {
	if len(message) >= 1 {
		messageTrytes, err := converter.ASCIIToTrytes(message[0])
		if err != nil {
			panic(err)
		}

		bundleFactory.outputs = append(bundleFactory.outputs, bundleFactoryOutputEntry{
			address: address,
			value:   value,
			message: trinary.MustPad(messageTrytes, value_transaction.SIGNATURE_MESSAGE_FRAGMENT_SIZE),
		})
	} else {
		bundleFactory.outputs = append(bundleFactory.outputs, bundleFactoryOutputEntry{
			address: address,
			value:   value,
		})
	}
}

func (bundleFactory *BundleFactory) GenerateBundle(branchTransactionHash trinary.Trytes, trunkTransactionHash trinary.Trytes) *Bundle {
	transactions := bundleFactory.generateTransactions()

	bundleHash := bundleFactory.signTransactions(transactions)

	bundleFactory.connectTransactions(transactions, branchTransactionHash, trunkTransactionHash)

	return &Bundle{
		essenceHash:  bundleHash,
		transactions: transactions,
	}
}

func (bundleFactory *BundleFactory) generateTransactions() []*value_transaction.ValueTransaction {
	transactions := make([]*value_transaction.ValueTransaction, 0)

	for _, input := range bundleFactory.inputs {
		transaction := value_transaction.New()
		transaction.SetValue(input.value)
		transaction.SetAddress(input.address.trytes)

		transactions = append(transactions, transaction)

		for i := 1; i < int(input.address.securityLevel); i++ {
			transaction := value_transaction.New()
			transaction.SetValue(0)
			transaction.SetAddress(input.address.trytes)

			transactions = append(transactions, transaction)
		}
	}

	for _, output := range bundleFactory.outputs {
		transaction := value_transaction.New()
		transaction.SetValue(output.value)
		transaction.SetAddress(output.address.trytes)

		if len(output.message) != 0 {
			transaction.SetSignatureMessageFragment(output.message)
		}

		transactions = append(transactions, transaction)
	}

	transactions[0].SetHead(true)
	transactions[len(transactions)-1].SetTail(true)

	return transactions
}

func (bundleFactory *BundleFactory) signTransactions(transactions []*value_transaction.ValueTransaction) trinary.Trytes {
	bundleHash := CalculateBundleHash(transactions)
	normalizedBundleHash := signing.NormalizedBundleHash(bundleHash)

	signedTransactions := 0
	for _, input := range bundleFactory.inputs {
		securityLevel := input.address.securityLevel
		privateKey := input.address.privateKey

		for i := 0; i < int(securityLevel); i++ {
			signedFragTrits, _ := signing.SignatureFragment(
				normalizedBundleHash[i*consts.HashTrytesSize/3:(i+1)*consts.HashTrytesSize/3],
				privateKey[i*consts.KeyFragmentLength:(i+1)*consts.KeyFragmentLength],
			)

			transactions[signedTransactions].SetSignatureMessageFragment(trinary.MustTritsToTrytes(signedFragTrits))

			signedTransactions++
		}
	}

	return bundleHash
}

func (bundleFactory *BundleFactory) connectTransactions(transactions []*value_transaction.ValueTransaction, branchTransactionHash trinary.Trytes, trunkTransactionHash trinary.Trytes) {
	transactionCount := len(transactions)

	transactions[transactionCount-1].SetBranchTransactionHash(branchTransactionHash)
	transactions[transactionCount-1].SetTrunkTransactionHash(trunkTransactionHash)

	for i := transactionCount - 2; i >= 0; i-- {
		transactions[i].SetBranchTransactionHash(branchTransactionHash)
		transactions[i].SetTrunkTransactionHash(transactions[i+1].GetHash())
	}
}

type bundleFactoryInputEntry struct {
	address *Address
	value   int64
}

type bundleFactoryOutputEntry struct {
	address *Address
	value   int64
	message trinary.Trytes
}
