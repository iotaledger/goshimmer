package faucet

import (
	"fmt"

	"github.com/iotaledger/hive.go/core/types/confirmation"
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle/models"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/packages/protocol"
	models2 "github.com/iotaledger/goshimmer/packages/protocol/instance/engine/congestioncontrol/icca/mana/manamodels"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm/indexer"

	"github.com/iotaledger/goshimmer/plugins/blocklayer"
)

type FaucetConnector struct {
	protocol *protocol.Protocol
	indexer  *indexer.Indexer
}

func NewConnector(p *protocol.Protocol, indexer *indexer.Indexer) *FaucetConnector {
	return &FaucetConnector{
		protocol: p,
		indexer:  indexer,
	}
}

func (f *FaucetConnector) UnspentOutputs(addresses ...address.Address) (unspentOutputs wallet.OutputsByAddressAndOutputID, err error) {
	unspentOutputs = make(map[address.Address]map[utxo.OutputID]*wallet.Output)

	for _, addr := range addresses {
		fmt.Println("> Getting unspent outputs for ", addr.Base58())
		f.indexer.CachedAddressOutputMappings(addr.Address()).Consume(func(mapping *indexer.AddressOutputMapping) {
			f.protocol.Instance().Engine.Ledger.Storage.CachedOutput(mapping.OutputID()).Consume(func(output utxo.Output) {
				fmt.Println("> > Found output ", output.String())
				if typedOutput, ok := output.(devnetvm.Output); ok {
					f.protocol.Instance().Engine.Ledger.Storage.CachedOutputMetadata(typedOutput.ID()).Consume(func(outputMetadata *ledger.OutputMetadata) {
						if !outputMetadata.IsSpent() {
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
	fmt.Printf("%+v\n", unspentOutputs)
	return
}

func (f *FaucetConnector) SendTransaction(tx *devnetvm.Transaction) (err error) {
	// attach to block layer
	issueTransaction := func() (*models.Block, error) {
		// TODO: finish when issuing blocks if implemented
		//block, e := deps.Tangle.IssuePayload(tx)
		//if e != nil {
		//	return nil, e
		//}
		//return block, nil
		return nil, nil
	}

	_, err = blocklayer.AwaitBlockToBeBooked(issueTransaction, tx.ID(), Parameters.MaxTransactionBookedAwaitTime)
	if err != nil {
		return errors.Errorf("%v: tx %s", err, tx.ID().String())
	}
	return nil
}

func (f *FaucetConnector) RequestFaucetFunds(address address.Address, powTarget int) (err error) {
	panic("RequestFaucetFunds is not implemented in faucet connector.")
}

func (f *FaucetConnector) GetAllowedPledgeIDs() (pledgeIDMap map[models2.Type][]string, err error) {
	pledgeIDMap = make(map[models2.Type][]string)
	pledgeIDMap[models2.AccessMana] = []string{deps.Local.ID().EncodeBase58()}
	pledgeIDMap[models2.ConsensusMana] = []string{deps.Local.ID().EncodeBase58()}

	return
}

func (f *FaucetConnector) GetTransactionConfirmationState(txID utxo.TransactionID) (confirmationState confirmation.State, err error) {
	f.protocol.Instance().Engine.Ledger.Storage.CachedTransactionMetadata(txID).Consume(func(tm *ledger.TransactionMetadata) {
		confirmationState = tm.ConfirmationState()
	})
	return
}

func (f *FaucetConnector) GetUnspentAliasOutput(address *devnetvm.AliasAddress) (output *devnetvm.AliasOutput, err error) {
	panic("GetUnspentAliasOutput is not implemented in faucet connector.")
}
