package faucet

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/client/wallet/packages/sendoptions"
	"github.com/iotaledger/goshimmer/packages/app/blockissuer"
	"github.com/iotaledger/goshimmer/packages/app/faucet"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm/indexer"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/bitmask"
)

type Faucet struct {
	*wallet.Wallet
}

// NewFaucet creates a new Faucet instance.
func NewFaucet(faucetSeed *seed.Seed, p *protocol.Protocol, issuer *blockissuer.BlockIssuer, indexer *indexer.Indexer) (f *Faucet) {
	connector := NewConnector(p, issuer, indexer)

	f = &Faucet{wallet.New(
		wallet.GenericConnector(connector),
		wallet.Import(faucetSeed, 0, []bitmask.BitMask{}, nil),
		wallet.ReusableAddress(true),
		wallet.FaucetPowDifficulty(Parameters.PowDifficulty),
		wallet.ConfirmationTimeout(Parameters.MaxAwait),
		wallet.ConfirmationPollingInterval(500*time.Millisecond),
		wallet.Stateless(true),
	)}
	// We use index 1 as a proxy address from which we send the funds to the requester.
	f.Wallet.NewReceiveAddress()

	return f
}

// Start starts the faucet to fulfill faucet requests.
func (f *Faucet) Start(ctx context.Context, requestChan <-chan *faucet.Payload) {
	for {
		select {
		case p := <-requestChan:
			tx, err := f.handleFaucetRequest(p, ctx)
			if err != nil {
				Plugin.LogErrorf("fail to send funds to %s: %v", p.Address().Base58(), err)
				continue
			}
			Plugin.LogInfof("sent funds to %s: TXID: %s", p.Address().Base58(), tx.ID().Base58())

		case <-ctx.Done():
			return
		}
	}
}

// handleFaucetRequest sends funds to the requested address and waits for the transaction to become accepted.
func (f *Faucet) handleFaucetRequest(p *faucet.Payload, ctx context.Context) (*devnetvm.Transaction, error) {
	_, err := f.SendFunds(
		sendoptions.Sources(f.Seed().Address(0)),                                          // we only reuse the address at index 0 for the wallet
		sendoptions.Destination(f.Seed().Address(1), uint64(Parameters.TokensPerRequest)), // we send the funds to address at index 1 so that we can be sure the correct output is sent to a requester
		sendoptions.AccessManaPledgeID(identity.ID{}.EncodeBase58()),
		sendoptions.ConsensusManaPledgeID(identity.ID{}.EncodeBase58()),
		sendoptions.WaitForConfirmation(true),
		sendoptions.Context(ctx),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to send first transaction from %s to %s", f.Seed().Address(0).Base58(), f.Seed().Address(1).Base58())
	}

	// send funds to requester
	tx, err := f.SendFunds(
		sendoptions.Sources(f.Seed().Address(1)),
		sendoptions.Destination(address.Address{AddressBytes: p.Address().Array()}, uint64(Parameters.TokensPerRequest)),
		sendoptions.AccessManaPledgeID(p.AccessManaPledgeID().EncodeBase58()),
		sendoptions.ConsensusManaPledgeID(p.ConsensusManaPledgeID().EncodeBase58()),
		sendoptions.WaitForConfirmation(true),
		sendoptions.Context(ctx),
	)
	return tx, errors.Wrapf(err, "failed to send second transaction from %s to %s", f.Seed().Address(1).Base58(), p.Address().Base58())
}
