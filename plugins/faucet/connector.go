package faucet

import (
	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm/indexer"
	"github.com/iotaledger/goshimmer/packages/core/mana"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/goshimmer/plugins/blocklayer"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/blocklayer"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types/confirmation"
	"github.com/mr-tron/base58"
	"github.com/pkg/errors"
)

type FaucetConnector struct {
	tangle  *tangleold.Tangle
	indexer *indexer.Indexer
}

func NewConnector(t *tangleold.Tangle, indexer *indexer.Indexer) *FaucetConnector {
	return &FaucetConnector{
		tangle:  t,
		indexer: indexer,
	}
}

func (f *FaucetConnector) UnspentOutputs(addresses ...address.Address) (unspentOutputs wallet.OutputsByAddressAndOutputID, err error) {
	unspentOutputs = make(map[address.Address]map[utxo.OutputID]*wallet.Output)

	for _, addr := range addresses {
		f.indexer.CachedAddressOutputMappings(addr.Address()).Consume(func(mapping *indexer.AddressOutputMapping) {
			f.tangle.Ledger.Storage.CachedOutput(mapping.OutputID()).Consume(func(output utxo.Output) {
				if typedOutput, ok := output.(devnetvm.Output); ok {
					f.tangle.Ledger.Storage.CachedOutputMetadata(typedOutput.ID()).Consume(func(outputMetadata *ledger.OutputMetadata) {
						if !outputMetadata.IsSpent() {
							addr := address.Address{AddressBytes: typedOutput.Address().Array()}
							walletOutput := &wallet.Output{
								Address:                  addr,
								Object:                   typedOutput,
								ConfirmationStateReached: outputMetadata.ConfirmationState().IsAccepted(),
								Spent:                    false,
								Metadata: wallet.OutputMetadata{
									Timestamp: outputMetadata.CreationTime(),
								},
							}

							// store output in result
							if _, addressExists := unspentOutputs[addr]; !addressExists {
								unspentOutputs[addr] = make(map[utxo.OutputID]*wallet.Output)
							}
							unspentOutputs[addr][typedOutput.ID()] = walletOutput
						}
					})
				}
			})
		})
	}

	return
}

func (f *FaucetConnector) SendTransaction(tx *devnetvm.Transaction) (err error) {
	// attach to block layer
	issueTransaction := func() (*tangleold.Block, error) {
		block, e := deps.Tangle.IssuePayload(tx)
		if e != nil {
			return nil, e
		}
		return block, nil
	}

	_, err = blocklayer.AwaitBlockToBeBooked(issueTransaction, tx.ID(), maxTxBookedAwaitTime)
	if err != nil {
		return errors.Errorf("%w: tx %s", err, tx.ID().String())
	}
	return nil
}

func (f *FaucetConnector) RequestFaucetFunds(address address.Address, powTarget int) (err error) {
	return nil
}

func (f *FaucetConnector) GetAllowedPledgeIDs() (pledgeIDMap map[mana.Type][]string, err error) {
	access := manaPlugin.GetAllowedPledgeNodes(mana.AccessMana)
	var accessNodes []string
	access.Allowed.ForEach(func(element identity.ID) {
		accessNodes = append(accessNodes, base58.Encode(element.Bytes()))
	})
	if len(accessNodes) == 0 {
		return nil, errors.New("No access mana pledge IDs are accepted")
	}

	consensus := manaPlugin.GetAllowedPledgeNodes(mana.ConsensusMana)
	var consensusNodes []string
	consensus.Allowed.ForEach(func(element identity.ID) {
		consensusNodes = append(consensusNodes, base58.Encode(element.Bytes()))
	})
	if len(consensusNodes) == 0 {
		return nil, errors.New("No consensus mana pledge IDs are accepted")
	}

	pledgeIDMap = map[mana.Type][]string{
		mana.AccessMana:    accessNodes,
		mana.ConsensusMana: consensusNodes,
	}

	return
}

func (f *FaucetConnector) GetTransactionConfirmationState(txID utxo.TransactionID) (confirmationState confirmation.State, err error) {
	Plugin.LogInfof("Start listening for confirmation")

	f.tangle.Ledger.Storage.CachedTransactionMetadata(txID).Consume(func(tm *ledger.TransactionMetadata) {
		confirmationState = tm.ConfirmationState()
	})
	return
}

func (f *FaucetConnector) GetUnspentAliasOutput(address *devnetvm.AliasAddress) (output *devnetvm.AliasOutput, err error) {
	return
}
