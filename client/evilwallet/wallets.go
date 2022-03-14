package evilwallet

import (
	"fmt"
	"sync"

	"github.com/cockroachdb/errors"

	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/types"
	"go.uber.org/atomic"
)

type walletID int
type WalletType int8
type WalletStatus int8

const (
	// fresh is used for automatic Faucet Requests, outputs are returned one by one
	other WalletType = iota
	fresh
)

// region Wallets ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Wallets struct {
	wallets map[walletID]*Wallet
	// we store here non-empty wallets ids of wallets with fresh faucet outputs.
	faucetWallets []walletID
	mu            sync.RWMutex

	lastWalletID atomic.Int64
}

func NewWallets() *Wallets {
	return &Wallets{
		wallets:       make(map[walletID]*Wallet),
		faucetWallets: make([]walletID, 0),
		lastWalletID:  *atomic.NewInt64(-1),
	}
}

func (w *Wallets) NewWallet(walletType WalletType) *Wallet {
	wallet := NewWallet(walletType)
	wallet.ID = walletID(w.lastWalletID.Add(1))

	w.addWallet(wallet)

	return wallet
}

func (w *Wallets) GetWallet(walletID walletID) *Wallet {
	return w.wallets[walletID]
}

// addWallet stores newly created wallet.
func (w *Wallets) addWallet(wallet *Wallet) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.wallets[wallet.ID] = wallet

}

// GetUnspentOutput gets first found unspent output for a given walletType
func (w *Wallets) GetUnspentOutput(wallet *Wallet) *Output {
	if wallet == nil {
		return nil
	}
	return wallet.GetUnspentOutput()
}

// IsFaucetWalletAvailable checks if there is any faucet wallet left.
func (w *Wallets) IsFaucetWalletAvailable() bool {
	return len(w.faucetWallets) > 0
}

// FreshWallet returns the first non-empty wallet from the faucetWallets queue. If current wallet is empty,
// it is removed and the next enqueued one is returned.
func (w *Wallets) FreshWallet() (*Wallet, error) {
	if !w.IsFaucetWalletAvailable() {
		return nil, errors.New("no faucet wallets available, need to request more funds")
	}
	wallet := w.wallets[w.faucetWallets[0]]
	if wallet.IsEmpty() {
		w.removeWallet(fresh)
		if !w.IsFaucetWalletAvailable() {
			return nil, errors.New("no faucet wallets available, need to request more funds")
		}
		// take next wallet
		wallet = w.wallets[w.faucetWallets[0]]
		fmt.Println("next fresh wallet ", wallet.ID)
		if wallet.IsEmpty() {
			return nil, errors.New("wallet is empty, need to request more funds")
		}
	}
	return wallet, nil
}

// removeWallet removes wallet, for fresh wallet it will be the first wallet in a queue.
func (w *Wallets) removeWallet(wType WalletType) {
	switch wType {
	case fresh:
		w.faucetWallets = w.faucetWallets[1:]

	}
	return
}

// SetWalletReady makes wallet ready to use, fresh wallet is added to freshWallets queue.
func (w *Wallets) SetWalletReady(wallet *Wallet) {
	wType := wallet.walletType
	switch wType {
	case fresh:
		w.faucetWallets = append(w.faucetWallets, wallet.ID)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////

// region Wallet ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Wallet struct {
	ID                walletID
	walletType        WalletType
	unspentOutputs    map[string]*Output // maps addr to its unspentOutput
	indexAddrMap      map[uint64]string
	addrIndexMap      map[string]uint64
	inputTransactions map[string]types.Empty
	seed              *seed.Seed

	lastAddrIdxUsed atomic.Int64 // used during filling in wallet with new outputs
	lastAddrSpent   atomic.Int64 // used during spamming with outputs one by one

	*sync.RWMutex
}

// NewWallet creates a wallet of a given type.
func NewWallet(wType ...WalletType) *Wallet {
	walletType := other
	if len(wType) > 0 {
		walletType = wType[0]
	}
	idxSpent := atomic.NewInt64(-1)
	addrUsed := atomic.NewInt64(-1)
	wallet := &Wallet{
		walletType:        walletType,
		ID:                -1,
		seed:              seed.NewSeed(),
		unspentOutputs:    make(map[string]*Output),
		indexAddrMap:      make(map[uint64]string),
		addrIndexMap:      make(map[string]uint64),
		inputTransactions: make(map[string]types.Empty),
		lastAddrSpent:     *idxSpent,
		lastAddrIdxUsed:   *addrUsed,
		RWMutex:           &sync.RWMutex{},
	}

	return wallet
}

// Address returns a new and unused address of a given wallet.
func (w *Wallet) Address() address.Address {
	w.Lock()
	defer w.Unlock()

	index := uint64(w.lastAddrIdxUsed.Add(1))
	addr := w.seed.Address(index)
	w.indexAddrMap[index] = addr.Base58()
	w.addrIndexMap[addr.Base58()] = index

	return addr
}

func (w *Wallet) UnspentOutput(addr string) *Output {
	w.RLock()
	defer w.RUnlock()
	return w.unspentOutputs[addr]

}

func (w *Wallet) UnspentOutputs() (outputs map[string]*Output) {
	w.RLock()
	outputs = make(map[string]*Output)
	for addr, out := range w.unspentOutputs {
		outputs[addr] = out
	}
	defer w.RUnlock()
	return outputs

}

func (w *Wallet) IndexAddrMap(outIndex uint64) string {
	w.RLock()
	defer w.RUnlock()

	return w.indexAddrMap[outIndex]
}

// AddUnspentOutput adds an unspentOutput of a given wallet.
func (w *Wallet) AddUnspentOutput(addr ledgerstate.Address, addrIdx uint64, outputID ledgerstate.OutputID, balance *ledgerstate.ColoredBalances) *Output {
	w.Lock()
	defer w.Unlock()

	out := &Output{
		OutputID: outputID,
		Address:  addr,
		Index:    addrIdx,
		Balance:  balance,
		Status:   pending,
	}
	w.unspentOutputs[addr.Base58()] = out
	return out
}

func (w *Wallet) UnspentOutputBalance(addr string) *ledgerstate.ColoredBalances {
	w.RLock()
	defer w.RUnlock()

	if out, ok := w.unspentOutputs[addr]; ok {
		return out.Balance
	}
	return &ledgerstate.ColoredBalances{}
}

func (w *Wallet) IsEmpty() bool {
	return w.lastAddrSpent.Load() == w.lastAddrIdxUsed.Load() || w.UnspentOutputsLength() == 0
}

func (w *Wallet) GetUnspentOutput() *Output {
	if w.lastAddrSpent.Load() < w.lastAddrIdxUsed.Load() {
		idx := w.lastAddrSpent.Add(1)
		addr := w.IndexAddrMap(uint64(idx))
		return w.UnspentOutput(addr)
	}
	return nil
}

func (w *Wallet) createOutputs(nOutputs int, inputBalance *ledgerstate.ColoredBalances) (outputs []ledgerstate.Output) {
	amount, _ := inputBalance.Get(ledgerstate.ColorIOTA)
	outputBalances := SplitBalanceEqually(nOutputs, amount)
	for i := 0; i < nOutputs; i++ {
		addr := w.Address()
		output := ledgerstate.NewSigLockedSingleOutput(outputBalances[i], addr.Address())
		outputs = append(outputs, output)
		// correct ID will be updated after txn confirmation
		w.AddUnspentOutput(addr.Address(), addr.Index, output.ID(), ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: outputBalances[i],
		}))
	}
	return
}

func (w *Wallet) sign(addr ledgerstate.Address, txEssence *ledgerstate.TransactionEssence) *ledgerstate.ED25519Signature {
	index := w.addrIndexMap[addr.Base58()]
	kp := w.seed.KeyPair(index)
	return ledgerstate.NewED25519Signature(kp.PublicKey, kp.PrivateKey.Sign(txEssence.Bytes()))
}

func (w *Wallet) UpdateUnspentOutputID(addr string, outputID ledgerstate.OutputID) error {
	w.RLock()
	walletOutput, ok := w.unspentOutputs[addr]
	w.RUnlock()
	if !ok {
		return errors.Newf("could not find unspent output under provided address in the wallet, outIdx:%d, addr: %s", outputID, addr)
	}
	w.Lock()
	walletOutput.OutputID = outputID
	w.Unlock()
	return nil
}

func (w *Wallet) UpdateUnspentOutputStatus(addr string, status OutputStatus) error {
	w.RLock()
	walletOutput, ok := w.unspentOutputs[addr]
	w.RUnlock()
	if !ok {
		return errors.Newf("could not find unspent output under provided address in the wallet, outIdx:%d, addr: %s", addr)
	}
	w.Lock()
	walletOutput.Status = status
	w.Unlock()
	return nil
}

func (w *Wallet) UnspentOutputsLength() int {
	return len(w.unspentOutputs)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////
