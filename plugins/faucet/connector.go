package faucet

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/packages/app/blockissuer"
	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm/indexer"
)

type Connector struct {
	blockIssuer *blockissuer.BlockIssuer
	protocol    *protocol.Protocol
	indexer     *indexer.Indexer
}

func NewConnector(p *protocol.Protocol, blockIssuer *blockissuer.BlockIssuer, indexer *indexer.Indexer) *Connector {
	return &Connector{
		blockIssuer: blockIssuer,
		protocol:    p,
		indexer:     indexer,
	}
}

func (f *Connector) UnspentOutputs(addresses ...address.Address) (unspentOutputs wallet.OutputsByAddressAndOutputID, err error) {
	unspentOutputs = make(map[address.Address]map[utxo.OutputID]*wallet.Output)

	for _, addr := range addresses {
		f.indexer.CachedAddressOutputMappings(addr.Address()).Consume(func(mapping *indexer.AddressOutputMapping) {
			f.protocol.Engine().Ledger.MemPool().Storage().CachedOutput(mapping.OutputID()).Consume(func(output utxo.Output) {
				if typedOutput, ok := output.(devnetvm.Output); ok {
					f.protocol.Engine().Ledger.MemPool().Storage().CachedOutputMetadata(typedOutput.ID()).Consume(func(outputMetadata *mempool.OutputMetadata) {
						if !outputMetadata.IsSpent() {
							walletOutput := &wallet.Output{
								Address:                  addr,
								Object:                   typedOutput,
								ConfirmationStateReached: outputMetadata.ConfirmationState().IsAccepted(),
								Spent:                    false,
								Metadata: wallet.OutputMetadata{
									Timestamp: f.protocol.SlotTimeProvider().EndTime(outputMetadata.InclusionSlot()),
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

func (f *Connector) SendTransaction(tx *devnetvm.Transaction) (err error) {
	block, err := f.blockIssuer.CreateBlock(tx)
	if err != nil {
		return errors.Wrapf(err, "error sending tx %s", tx.ID().String())
	}

	err = f.blockIssuer.IssueBlockAndAwaitBlockToBeBooked(block, Parameters.MaxTransactionBookedAwaitTime)
	if err != nil {
		return errors.Wrapf(err, "error sending tx %s", tx.ID().String())
	}

	return nil
}

func (f *Connector) RequestFaucetFunds(address address.Address, powTarget int) (err error) {
	panic("RequestFaucetFunds is not implemented in faucet connector.")
}

func (f *Connector) GetTransactionConfirmationState(txID utxo.TransactionID) (confirmationState confirmation.State, err error) {
	f.protocol.Engine().Ledger.MemPool().Storage().CachedTransactionMetadata(txID).Consume(func(tm *mempool.TransactionMetadata) {
		confirmationState = tm.ConfirmationState()
	})
	return
}

func (f *Connector) GetUnspentAliasOutput(address *devnetvm.AliasAddress) (output *devnetvm.AliasOutput, err error) {
	panic("GetUnspentAliasOutput is not implemented in faucet connector.")
}
