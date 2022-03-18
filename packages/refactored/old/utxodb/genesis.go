package utxodb

import (
	"math/rand"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/refactored/txvm"
	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

const (
	defaultSupplyInt = 2779530283000000
	defaultSupply    = uint64(defaultSupplyInt)
	genesisIndex     = 31415926535

	// RequestFundsAmount is how many iotas are returned from the faucet.
	RequestFundsAmount = 1000000 // 1Mi
)

var essenceVersion = txvm.TransactionEssenceVersion(0)

// UtxoDB is the structure which contains all UTXODB transactions and ledger.
type UtxoDB struct {
	seed            *ed25519.Seed
	supply          uint64
	genesisKeyPair  *ed25519.KeyPair
	genesisAddress  txvm.Address
	transactions    map[utxo.TransactionID]*txvm.Transaction
	utxo            map[utxo.OutputID]txvm.Output
	consumedOutputs map[utxo.OutputID]txvm.Output
	consumedBy      map[utxo.OutputID]utxo.TransactionID
	mutex           *sync.RWMutex
	genesisTxID     utxo.TransactionID
}

// New creates new UTXODB instance.
func newUtxodb(seed *ed25519.Seed, supply uint64, timestamp time.Time) *UtxoDB {
	genesisKeyPair := seed.KeyPair(uint64(genesisIndex))
	genesisAddress := txvm.NewED25519Address(genesisKeyPair.PublicKey)
	u := &UtxoDB{
		seed:            seed,
		supply:          supply,
		genesisKeyPair:  genesisKeyPair,
		genesisAddress:  genesisAddress,
		transactions:    make(map[utxo.TransactionID]*txvm.Transaction),
		utxo:            make(map[utxo.OutputID]txvm.Output),
		consumedOutputs: make(map[utxo.OutputID]txvm.Output),
		consumedBy:      make(map[utxo.OutputID]utxo.TransactionID),
		mutex:           &sync.RWMutex{},
	}
	u.genesisInit(timestamp)
	return u
}

// New creates new utxodb instance with predefined genesis seed and optional supply.
// Supply defaults to the standard IOTA supply.
func New(supply ...uint64) *UtxoDB {
	s := defaultSupply
	if len(supply) > 0 {
		s = supply[0]
	}
	return newUtxodb(ed25519.NewSeed([]byte("EFonzaUz5ngYeDxbRKu8qV5aoSogUQ5qVSTSjn7hJ8FQ")), s, time.Now())
}

func NewWithTimestamp(timestamp time.Time, supply ...uint64) *UtxoDB {
	s := defaultSupply
	if len(supply) > 0 {
		s = supply[0]
	}
	return newUtxodb(ed25519.NewSeed([]byte("EFonzaUz5ngYeDxbRKu8qV5aoSogUQ5qVSTSjn7hJ8FQ")), s, timestamp)
}

// NewRandom creates utxodb with random genesis seed.
func NewRandom(supply ...uint64) *UtxoDB {
	s := defaultSupply
	if len(supply) > 0 {
		s = supply[0]
	}
	var rnd [32]byte
	rand.Read(rnd[:])
	return newUtxodb(ed25519.NewSeed(rnd[:]), s, time.Now())
}

// NewKeyPairByIndex creates key pair and address generated from the seed and the index.
func (u *UtxoDB) NewKeyPairByIndex(index int) (*ed25519.KeyPair, *txvm.ED25519Address) {
	kp := u.seed.KeyPair(uint64(index))
	return kp, txvm.NewED25519Address(kp.PublicKey)
}

func (u *UtxoDB) genesisInit(timestamp time.Time) {
	// create genesis transaction
	inputs := txvm.NewInputs(txvm.NewUTXOInput(utxo.NewOutputID(utxo.TransactionID{}, 0, []byte(""))))
	output := txvm.NewSigLockedSingleOutput(defaultSupply, u.GetGenesisAddress())
	outputs := txvm.NewOutputs(output)
	essence := txvm.NewTransactionEssence(essenceVersion, timestamp, identity.ID{}, identity.ID{}, inputs, outputs)
	signature := txvm.NewED25519Signature(u.genesisKeyPair.PublicKey, u.genesisKeyPair.PrivateKey.Sign(essence.Bytes()))
	unlockBlock := txvm.NewSignatureUnlockBlock(signature)
	genesisTx := txvm.NewTransaction(essence, txvm.UnlockBlocks{unlockBlock})

	u.genesisTxID = genesisTx.ID()
	u.transactions[u.genesisTxID] = genesisTx
	u.utxo[output.ID()] = output.Clone()
}

func (u *UtxoDB) GenesisTransactionID() utxo.TransactionID {
	return u.genesisTxID
}

// GetGenesisKeyPair return signature scheme used by creator of genesis.
func (u *UtxoDB) GetGenesisKeyPair() *ed25519.KeyPair {
	return u.genesisKeyPair
}

// GetGenesisAddress return address of genesis.
func (u *UtxoDB) GetGenesisAddress() txvm.Address {
	return u.genesisAddress
}

func (u *UtxoDB) mustRequestFundsTx(target txvm.Address, timestamp time.Time) *txvm.Transaction {
	sourceOutputs := u.GetAddressOutputs(u.GetGenesisAddress())
	if len(sourceOutputs) != 1 {
		panic("number of genesis outputs must be 1")
	}
	remainder, _ := sourceOutputs[0].Balances().Get(txvm.ColorIOTA)
	o1 := txvm.NewSigLockedSingleOutput(RequestFundsAmount, target)
	o2 := txvm.NewSigLockedSingleOutput(remainder-RequestFundsAmount, u.GetGenesisAddress())
	outputs := txvm.NewOutputs(o1, o2)
	inputs := txvm.NewInputs(txvm.NewUTXOInput(sourceOutputs[0].ID()))
	essence := txvm.NewTransactionEssence(0, timestamp, identity.ID{}, identity.ID{}, inputs, outputs)
	signature := txvm.NewED25519Signature(u.genesisKeyPair.PublicKey, u.genesisKeyPair.PrivateKey.Sign(essence.Bytes()))
	unlockBlocks := []txvm.UnlockBlock{txvm.NewSignatureUnlockBlock(signature)}
	return txvm.NewTransaction(essence, unlockBlocks)
}

// RequestFunds implements faucet: it sends 1337 IOTA tokens from genesis to the given address.
func (u *UtxoDB) RequestFunds(target txvm.Address, timestamp ...time.Time) (*txvm.Transaction, error) {
	t := time.Now()
	if len(timestamp) > 0 {
		t = timestamp[0]
	}
	tx := u.mustRequestFundsTx(target, t)
	return tx, u.AddTransaction(tx)
}
