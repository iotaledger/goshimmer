package utxodb

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
)

const (
	Supply       = uint64(100 * 1000 * 1000)
	genesisIndex = 31415926535

	RequestFundsAmount = 1337 // same as Goshimmer faucet
)

var (
	seed           = ed25519.NewSeed([]byte("EFonzaUz5ngYeDxbRKu8qV5aoSogUQ5qVSTSjn7hJ8FQ"))
	genesisKeyPair = NewKeyPairFromSeed(genesisIndex)
	essenceVersion = ledgerstate.TransactionEssenceVersion(0)
)

// UtxoDB is the structure which contains all UTXODB transactions and ledger
type UtxoDB struct {
	genesisKeyPair *ed25519.KeyPair
	transactions   map[ledgerstate.TransactionID]*ledgerstate.Transaction
	utxo           map[ledgerstate.OutputID]ledgerstate.Output
	mutex          *sync.RWMutex
	genesisTxId    ledgerstate.TransactionID
}

// New creates new UTXODB instance
func New() *UtxoDB {
	u := &UtxoDB{
		genesisKeyPair: genesisKeyPair,
		transactions:   make(map[ledgerstate.TransactionID]*ledgerstate.Transaction),
		utxo:           make(map[ledgerstate.OutputID]ledgerstate.Output),
		mutex:          &sync.RWMutex{},
	}
	u.genesisInit()
	return u
}

func NewKeyPairFromSeed(index int) *ed25519.KeyPair {
	return seed.KeyPair(uint64(index))
}

func (u *UtxoDB) genesisInit() {
	// create genesis transaction
	inputs := ledgerstate.NewInputs(ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(ledgerstate.TransactionID{}, 0)))
	output := ledgerstate.NewSigLockedSingleOutput(Supply, u.GetGenesisAddress())
	outputs := ledgerstate.NewOutputs(output)
	essence := ledgerstate.NewTransactionEssence(essenceVersion, time.Now(), identity.ID{}, identity.ID{}, inputs, outputs)
	signature := ledgerstate.NewED25519Signature(u.genesisKeyPair.PublicKey, u.genesisKeyPair.PrivateKey.Sign(essence.Bytes()))
	unlockBlock := ledgerstate.NewSignatureUnlockBlock(signature)
	genesisTx := ledgerstate.NewTransaction(essence, ledgerstate.UnlockBlocks{unlockBlock})

	u.genesisTxId = genesisTx.ID()
	u.transactions[u.genesisTxId] = genesisTx
	u.utxo[output.ID()] = output.Clone()
}

// GetGenesisSigScheme return signature scheme used by creator of genesis
func (u *UtxoDB) GetGenesisKeyPair() *ed25519.KeyPair {
	return genesisKeyPair
}

// GetGenesisAddress return address of genesis
func (u *UtxoDB) GetGenesisAddress() ledgerstate.Address {
	return ledgerstate.NewED25519Address(genesisKeyPair.PublicKey)
}

func (u *UtxoDB) mustRequestFundsTx(target ledgerstate.Address) *ledgerstate.Transaction {
	sourceOutputs := u.GetAddressOutputs(u.GetGenesisAddress())
	if len(sourceOutputs) != 1 {
		panic("number of genesis outputs must be 1")
	}
	reminder, _ := sourceOutputs[0].Balances().Get(ledgerstate.ColorIOTA)
	o1 := ledgerstate.NewSigLockedSingleOutput(RequestFundsAmount, target)
	o2 := ledgerstate.NewSigLockedSingleOutput(reminder-RequestFundsAmount, u.GetGenesisAddress())
	outputs := ledgerstate.NewOutputs(o1, o2)
	inputs := ledgerstate.NewInputs(ledgerstate.NewUTXOInput(sourceOutputs[0].ID()))
	essence := ledgerstate.NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, inputs, outputs)
	signature := ledgerstate.NewED25519Signature(genesisKeyPair.PublicKey, genesisKeyPair.PrivateKey.Sign(essence.Bytes()))
	unlockBlocks := []ledgerstate.UnlockBlock{ledgerstate.NewSignatureUnlockBlock(signature)}
	return ledgerstate.NewTransaction(essence, unlockBlocks)
}

// RequestFunds implements faucet: it sends 1337 IOTA tokens from genesis to the given address.
func (u *UtxoDB) RequestFunds(target ledgerstate.Address) (*ledgerstate.Transaction, error) {
	tx := u.mustRequestFundsTx(target)
	return tx, u.AddTransaction(tx)
}
