package faucet

import (
	"context"
	"crypto/ed25519"
	"os"
	"time"
	"unsafe"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/client/wallet/packages/sendoptions"
	"github.com/iotaledger/goshimmer/packages/app/faucet"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/hive.go/core/bitmask"
	"github.com/iotaledger/hive.go/core/marshalutil"
)

var (
	maxTxBookedAwaitTime = 5 * time.Second
)

// remainder stays on index 0
type Faucet struct {
	*wallet.Wallet
}

// NewFaucet creates a new Faucet instance.
func NewFaucet(faucetSeed *seed.Seed, walletStates string) *Faucet {
	seed, lastAddressIndex, spentAddresses, assetRegistry, err := importWalletStateFile(walletStates)
	if err != nil {
		if !os.IsNotExist(err) {
			panic(err)
		}

		seed = faucetSeed
		lastAddressIndex = 0
		spentAddresses = []bitmask.BitMask{}
	}

	if faucetSeed.String() != seed.String() {
		panic("faucet seed is different from the one in wallet states file")
	}

	connector := NewConnector(deps.Tangle, deps.Indexer)

	return &Faucet{wallet.New(
		wallet.GenericConnector(connector),
		wallet.Import(seed, lastAddressIndex, spentAddresses, assetRegistry),
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
			Plugin.LogInfof("send funds to %s: TXID: %s", p.Address().Base58(), tx.ID().Base58())

		case <-ctx.Done():
			return
		}
	}
}

func (f *Faucet) handleFaucetRequest(p *faucet.Payload) (*devnetvm.Transaction, error) {
	return f.SendFunds(
		sendoptions.Destination(address.Address{AddressBytes: p.Address().Array()}, uint64(Parameters.TokensPerRequest)),
		sendoptions.AccessManaPledgeID(p.AccessManaPledgeID().String()),
		sendoptions.ConsensusManaPledgeID(p.ConsensusManaPledgeID().String()),
	)
}

func importWalletStateFile(filename string) (seed *seed.Seed, lastAddressIndex uint64, spentAddresses []bitmask.BitMask, assetRegistry *wallet.AssetRegistry, err error) {
	walletStateBytes, err := os.ReadFile(filename)
	if err != nil {
		return
	}

	marshalUtil := marshalutil.New(walletStateBytes)

	seedBytes, err := marshalUtil.ReadBytes(ed25519.SeedSize)
	seed = walletseed.NewSeed(seedBytes)
	if err != nil {
		return
	}

	lastAddressIndex, err = marshalUtil.ReadUint64()
	if err != nil {
		return
	}

	_, _, err = wallet.ParseAssetRegistry(marshalUtil)

	spentAddressesBytes := marshalUtil.ReadRemainingBytes()
	spentAddresses = *(*[]bitmask.BitMask)(unsafe.Pointer(&spentAddressesBytes))

	return
}

func writeWalletStateFile(wallet *wallet.Wallet, filename string) {
	info, err := os.Stat(filename)
	if err != nil {
		if !os.IsNotExist(err) {
			panic(err)
		}
	}
	if err == nil && info.IsDir() {
		panic("found directory instead of file at " + filename)
	}

	err = os.WriteFile(filename, wallet.ExportState(), 0o644)
	if err != nil {
		panic(err)
	}
}
