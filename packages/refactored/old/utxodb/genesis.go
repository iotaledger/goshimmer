package utxodb

import (
	"math/rand"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"

	utxo2 "github.com/iotaledger/goshimmer/packages/refactored/ledger/utxo"
	txvm2 "github.com/iotaledger/goshimmer/packages/refactored/ledger/vms/txvm"
)

const (
	defaultSupplyInt = 2779530283000000
	defaultSupply    = uint64(defaultSupplyInt)
	genesisIndex     = 31415926535

	// RequestFundsAmount is how many iotas are returned from the faucet.
	RequestFundsAmount = 1000000 // 1Mi
)

var essenceVersion = txvm2.TransactionEssenceVersion(0)

// UtxoDB is the structure which contains all UTXODB transactions and ledger.
type UtxoDB struct {
	seed            *ed25519.Seed
	supply          uint64
	genesisKeyPair  *ed25519.KeyPair
	genesisAddress  txvm2.Address
	transactions    map[utxo2.TransactionID]*txvm2.Transaction
	utxo            map[utxo2.OutputID]txvm2.OutputEssence
	consumedOutputs map[utxo2.OutputID]txvm2.OutputEssence
	consumedBy      map[utxo2.OutputID]utxo2.TransactionID
	mutex           *sync.RWMutex
	genesisTxID     utxo2.TransactionID
}

// New creates new UTXODB instance.
func newUtxodb(seed *ed25519.Seed, supply uint64, timestamp time.Time) *UtxoDB {
	genesisKeyPair := seed.KeyPair(uint64(genesisIndex))
	genesisAddress := txvm2.NewED25519Address(genesisKeyPair.PublicKey)
	u := &UtxoDB{
		seed:            seed,
		supply:          supply,
		genesisKeyPair:  genesisKeyPair,
		genesisAddress:  genesisAddress,
		transactions:    make(map[utxo2.TransactionID]*txvm2.Transaction),
		utxo:            make(map[utxo2.OutputID]txvm2.OutputEssence),
		consumedOutputs: make(map[utxo2.OutputID]txvm2.OutputEssence),
		consumedBy:      make(map[utxo2.OutputID]utxo2.TransactionID),
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
func (u *UtxoDB) NewKeyPairByIndex(index int) (*ed25519.KeyPair, *txvm2.ED25519Address) {
	kp := u.seed.KeyPair(uint64(index))
	return kp, txvm2.NewED25519Address(kp.PublicKey)
}

func (u *UtxoDB) genesisInit(timestamp time.Time) {
	// create genesis transaction
	inputs := txvm2.NewInputs(txvm2.NewUTXOInput(utxo2.NewOutputID(utxo2.TransactionID{}, 0, []byte(""))))
	output := txvm2.NewSigLockedSingleOutput(defaultSupply, u.GetGenesisAddress())
	outputs := txvm2.NewOutputs(output)
	essence := txvm2.NewTransactionEssence(essenceVersion, timestamp, identity.ID{}, identity.ID{}, inputs, outputs)
	signature := txvm2.NewED25519Signature(u.genesisKeyPair.PublicKey, u.genesisKeyPair.PrivateKey.Sign(essence.Bytes()))
	unlockBlock := txvm2.NewSignatureUnlockBlock(signature)
	genesisTx := txvm2.NewTransaction(essence, txvm2.UnlockBlocks{unlockBlock})

	u.genesisTxID = genesisTx.ID()
	u.transactions[u.genesisTxID] = genesisTx
	u.utxo[output.ID()] = output.Clone()
}

func (u *UtxoDB) GenesisTransactionID() utxo2.TransactionID {
	return u.genesisTxID
}

// GetGenesisKeyPair return signature scheme used by creator of genesis.
func (u *UtxoDB) GetGenesisKeyPair() *ed25519.KeyPair {
	return u.genesisKeyPair
}

// GetGenesisAddress return address of genesis.
func (u *UtxoDB) GetGenesisAddress() txvm2.Address {
	return u.genesisAddress
}

func (u *UtxoDB) mustRequestFundsTx(target txvm2.Address, timestamp time.Time) *txvm2.Transaction {
	sourceOutputs := u.GetAddressOutputs(u.GetGenesisAddress())
	if len(sourceOutputs) != 1 {
		panic("number of genesis outputs must be 1")
	}
	remainder, _ := sourceOutputs[0].Balances().Get(txvm2.ColorIOTA)
	o1 := txvm2.NewSigLockedSingleOutput(RequestFundsAmount, target)
	o2 := txvm2.NewSigLockedSingleOutput(remainder-RequestFundsAmount, u.GetGenesisAddress())
	outputs := txvm2.NewOutputs(o1, o2)
	inputs := txvm2.NewInputs(txvm2.NewUTXOInput(sourceOutputs[0].ID()))
	essence := txvm2.NewTransactionEssence(0, timestamp, identity.ID{}, identity.ID{}, inputs, outputs)
	signature := txvm2.NewED25519Signature(u.genesisKeyPair.PublicKey, u.genesisKeyPair.PrivateKey.Sign(essence.Bytes()))
	unlockBlocks := []txvm2.UnlockBlock{txvm2.NewSignatureUnlockBlock(signature)}
	return txvm2.NewTransaction(essence, unlockBlocks)
}

// RequestFunds implements faucet: it sends 1337 IOTA tokens from genesis to the given address.
func (u *UtxoDB) RequestFunds(target txvm2.Address, timestamp ...time.Time) (*txvm2.Transaction, error) {
	t := time.Now()
	if len(timestamp) > 0 {
		t = timestamp[0]
	}
	tx := u.mustRequestFundsTx(target, t)
	return tx, u.AddTransaction(tx)
}
