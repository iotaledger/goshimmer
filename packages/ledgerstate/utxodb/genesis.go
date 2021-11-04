package utxodb

import (
	"math/rand"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

const (
	defaultSupplyInt = 2779530283000000
	defaultSupply    = uint64(defaultSupplyInt)
	genesisIndex     = 31415926535

	// RequestFundsAmount is how many iotas are returned from the faucet.
	RequestFundsAmount = 1000000 // 1Mi
)

var essenceVersion = ledgerstate.TransactionEssenceVersion(0)

// UtxoDB is the structure which contains all UTXODB transactions and ledger.
type UtxoDB struct {
	seed            *ed25519.Seed
	supply          uint64
	genesisKeyPair  *ed25519.KeyPair
	genesisAddress  ledgerstate.Address
	transactions    map[ledgerstate.TransactionID]*ledgerstate.Transaction
	utxo            map[ledgerstate.OutputID]ledgerstate.Output
	consumedOutputs map[ledgerstate.OutputID]ledgerstate.Output
	consumedBy      map[ledgerstate.OutputID]ledgerstate.TransactionID
	mutex           *sync.RWMutex
	genesisTxID     ledgerstate.TransactionID
}

// New creates new UTXODB instance.
func newUtxodb(seed *ed25519.Seed, supply uint64, timestamp time.Time) *UtxoDB {
	genesisKeyPair := seed.KeyPair(uint64(genesisIndex))
	genesisAddress := ledgerstate.NewED25519Address(genesisKeyPair.PublicKey)
	u := &UtxoDB{
		seed:            seed,
		supply:          supply,
		genesisKeyPair:  genesisKeyPair,
		genesisAddress:  genesisAddress,
		transactions:    make(map[ledgerstate.TransactionID]*ledgerstate.Transaction),
		utxo:            make(map[ledgerstate.OutputID]ledgerstate.Output),
		consumedOutputs: make(map[ledgerstate.OutputID]ledgerstate.Output),
		consumedBy:      make(map[ledgerstate.OutputID]ledgerstate.TransactionID),
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
func (u *UtxoDB) NewKeyPairByIndex(index int) (*ed25519.KeyPair, *ledgerstate.ED25519Address) {
	kp := u.seed.KeyPair(uint64(index))
	return kp, ledgerstate.NewED25519Address(kp.PublicKey)
}

func (u *UtxoDB) genesisInit(timestamp time.Time) {
	// create genesis transaction
	inputs := ledgerstate.NewInputs(ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(ledgerstate.TransactionID{}, 0)))
	output := ledgerstate.NewSigLockedSingleOutput(defaultSupply, u.GetGenesisAddress())
	outputs := ledgerstate.NewOutputs(output)
	essence := ledgerstate.NewTransactionEssence(essenceVersion, timestamp, identity.ID{}, identity.ID{}, inputs, outputs)
	signature := ledgerstate.NewED25519Signature(u.genesisKeyPair.PublicKey, u.genesisKeyPair.PrivateKey.Sign(essence.Bytes()))
	unlockBlock := ledgerstate.NewSignatureUnlockBlock(signature)
	genesisTx := ledgerstate.NewTransaction(essence, ledgerstate.UnlockBlocks{unlockBlock})

	u.genesisTxID = genesisTx.ID()
	u.transactions[u.genesisTxID] = genesisTx
	u.utxo[output.ID()] = output.Clone()
}

func (u *UtxoDB) GenesisTransactionID() ledgerstate.TransactionID {
	return u.genesisTxID
}

// GetGenesisKeyPair return signature scheme used by creator of genesis.
func (u *UtxoDB) GetGenesisKeyPair() *ed25519.KeyPair {
	return u.genesisKeyPair
}

// GetGenesisAddress return address of genesis.
func (u *UtxoDB) GetGenesisAddress() ledgerstate.Address {
	return u.genesisAddress
}

func (u *UtxoDB) mustRequestFundsTx(target ledgerstate.Address, timestamp time.Time) *ledgerstate.Transaction {
	sourceOutputs := u.GetAddressOutputs(u.GetGenesisAddress())
	if len(sourceOutputs) != 1 {
		panic("number of genesis outputs must be 1")
	}
	remainder, _ := sourceOutputs[0].Balances().Get(ledgerstate.ColorIOTA)
	o1 := ledgerstate.NewSigLockedSingleOutput(RequestFundsAmount, target)
	o2 := ledgerstate.NewSigLockedSingleOutput(remainder-RequestFundsAmount, u.GetGenesisAddress())
	outputs := ledgerstate.NewOutputs(o1, o2)
	inputs := ledgerstate.NewInputs(ledgerstate.NewUTXOInput(sourceOutputs[0].ID()))
	essence := ledgerstate.NewTransactionEssence(0, timestamp, identity.ID{}, identity.ID{}, inputs, outputs)
	signature := ledgerstate.NewED25519Signature(u.genesisKeyPair.PublicKey, u.genesisKeyPair.PrivateKey.Sign(essence.Bytes()))
	unlockBlocks := []ledgerstate.UnlockBlock{ledgerstate.NewSignatureUnlockBlock(signature)}
	return ledgerstate.NewTransaction(essence, unlockBlocks)
}

// RequestFunds implements faucet: it sends 1337 IOTA tokens from genesis to the given address.
func (u *UtxoDB) RequestFunds(target ledgerstate.Address, timestamp ...time.Time) (*ledgerstate.Transaction, error) {
	t := time.Now()
	if len(timestamp) > 0 {
		t = timestamp[0]
	}
	tx := u.mustRequestFundsTx(target, t)
	return tx, u.AddTransaction(tx)
}
