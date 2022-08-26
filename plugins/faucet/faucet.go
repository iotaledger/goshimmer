package faucet

import (
	"context"
	"time"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/client/wallet/packages/sendoptions"
	"github.com/iotaledger/goshimmer/packages/app/faucet"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/hive.go/core/bitmask"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/pkg/errors"
)

// remainder stays on index 0
type Faucet struct {
	*wallet.Wallet
}

// NewFaucet creates a new Faucet instance.
func NewFaucet(faucetSeed *seed.Seed) *Faucet {
	connector := NewConnector(deps.Tangle, deps.Indexer)

	return &Faucet{wallet.New(
		wallet.GenericConnector(connector),
		wallet.Import(faucetSeed, 0, []bitmask.BitMask{}, nil),
		wallet.ReusableAddress(true),
		wallet.FaucetPowDifficulty(Parameters.PowDifficulty),
	)}
}

// Start starts the faucet to fulfill faucet requests.
func (f *Faucet) Start(ctx context.Context, requestChan <-chan *faucet.Payload) {
	for {
		select {
		case p := <-requestChan:
			tx, err := f.handleFaucetRequest(p)
			if err != nil {
				Plugin.LogErrorf("fail to send funds to %s: %v", p.Address().Base58(), err)
				return
			}
			Plugin.LogInfof("sent funds to %s: TXID: %s", p.Address().Base58(), tx.ID().Base58())

		case <-ctx.Done():
			return
		}
	}
}

// handleFaucetRequest sends funds to the requested address and waits for the transaction to become accepted.
func (f *Faucet) handleFaucetRequest(p *faucet.Payload) (*devnetvm.Transaction, error) {
	// send funds to faucet in order to pledge mana
	totalBalances := uint64(0)
	confirmed, _, err := f.Balance(true)
	if err != nil {
		return nil, err
	}
	for _, b := range confirmed {
		totalBalances += b
	}

	_, err = f.sendTransaction(
		f.Seed().Address(0), // we only reuse the address at index 0 for the wallet
		totalBalances-uint64(Parameters.TokensPerRequest),
		deps.Local.ID(),
		identity.ID{},
	)
	if err != nil {
		return nil, err
	}

	// send funds to requester
	tx, err := f.sendTransaction(
		address.Address{AddressBytes: p.Address().Array()},
		uint64(Parameters.TokensPerRequest),
		p.AccessManaPledgeID(),
		p.ConsensusManaPledgeID(),
	)
	return tx, err
}

func (f *Faucet) sendTransaction(destAddr address.Address, balance uint64, aManaPledgeID, cManaPledgeID identity.ID) (*devnetvm.Transaction, error) {
	txAccepted := make(chan utxo.TransactionID, 10)
	monitorTxAcceptance := event.NewClosure(func(event *ledger.TransactionAcceptedEvent) {
		txAccepted <- event.TransactionID
	})

	// listen on confirmation
	deps.Tangle.Ledger.Events.TransactionAccepted.Attach(monitorTxAcceptance)
	defer deps.Tangle.Ledger.Events.TransactionAccepted.Detach(monitorTxAcceptance)

	tx, err := f.SendFunds(
		sendoptions.Destination(destAddr, balance),
		sendoptions.AccessManaPledgeID(aManaPledgeID.EncodeBase58()),
		sendoptions.ConsensusManaPledgeID(cManaPledgeID.EncodeBase58()),
	)
	if err != nil {
		return nil, err
	}

	ticker := time.NewTicker(Parameters.MaxAwait)
	defer ticker.Stop()
	for {
		select {
		case txID := <-txAccepted:
			if tx.ID() == txID {
				return tx, nil
			}
		case <-ticker.C:
			return nil, errors.Errorf("TX %s is not confirmed in time", tx.ID())
		}
	}
}
