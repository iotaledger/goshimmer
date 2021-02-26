package faucet

import (
	"sync"
	"time"

	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/datastructure/orderedmap"
	"github.com/iotaledger/hive.go/identity"
	"golang.org/x/xerrors"
)

// New creates a new faucet component using the given seed and tokensPerRequest config.
func New(seed []byte, tokensPerRequest int64, blacklistCapacity int, maxTxBookedAwaitTime time.Duration) *Component {
	return &Component{
		tokensPerRequest:     tokensPerRequest,
		seed:                 walletseed.NewSeed(seed),
		maxTxBookedAwaitTime: maxTxBookedAwaitTime,
		blacklist:            orderedmap.New(),
		blacklistCapacity:    blacklistCapacity,
	}
}

// Component implements a faucet component which will send tokens to actors requesting tokens.
type Component struct {
	sync.Mutex
	// the amount of tokens to send to every request
	tokensPerRequest int64
	// the seed instance of the faucet holding the tokens
	seed *walletseed.Seed
	// the time to await for the transaction fulfilling a funding request
	// to become booked in the value layer
	maxTxBookedAwaitTime time.Duration
	blacklistCapacity    int
	blacklist            *orderedmap.OrderedMap
}

// IsAddressBlacklisted checks whether the given address is currently blacklisted.
func (c *Component) IsAddressBlacklisted(addr ledgerstate.Address) bool {
	_, blacklisted := c.blacklist.Get(addr)
	return blacklisted
}

// adds the given address to the blacklist and removes the oldest blacklist entry
// if it would go over capacity.
func (c *Component) addAddressToBlacklist(addr ledgerstate.Address) {
	c.blacklist.Set(addr, true)
	if c.blacklist.Size() > c.blacklistCapacity {
		var headKey interface{}
		c.blacklist.ForEach(func(key, value interface{}) bool {
			headKey = key
			return false
		})
		c.blacklist.Delete(headKey)
	}
}

// SendFunds sends IOTA tokens to the address from faucet request.
func (c *Component) SendFunds(msg *tangle.Message) (m *tangle.Message, txID string, err error) {
	// ensure that only one request is being processed any given time
	c.Lock()
	defer c.Unlock()

	addr := msg.Payload().(*Request).Address()

	if c.IsAddressBlacklisted(addr) {
		return nil, "", ErrAddressIsBlacklisted
	}

	// get the outputs for the inputs and remainder balance
	inputs, addrsIndices, remainder := c.collectUTXOsForFunding()
	var outputs ledgerstate.Outputs
	output := ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
		ledgerstate.ColorIOTA: uint64(c.tokensPerRequest),
	}), addr)
	outputs = append(outputs, output)
	// add remainder address if needed
	if remainder > 0 {
		remainAddr := c.nextUnusedAddress()
		output := ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: uint64(remainder),
		}), remainAddr)
		outputs = append(outputs, output)
	}

	txEssence := ledgerstate.NewTransactionEssence(0, clock.SyncedTime(), identity.ID{}, identity.ID{}, ledgerstate.NewInputs(inputs...), ledgerstate.NewOutputs(outputs...))

	//  TODO: check this
	unlockBlocks := make([]ledgerstate.UnlockBlock, len(txEssence.Inputs()))
	inputIndex := 0
	for i, inputs := range addrsIndices {
		w := wallet{
			keyPair: *c.seed.KeyPair(i),
		}
		for range inputs {
			unlockBlock := ledgerstate.NewSignatureUnlockBlock(w.sign(txEssence))
			unlockBlocks[inputIndex] = unlockBlock
			inputIndex++
		}
	}

	tx := ledgerstate.NewTransaction(txEssence, unlockBlocks)

	// attach to message layer
	issueTransaction := func() (*tangle.Message, error) {
		message, e := issuer.IssuePayload(tx, messagelayer.Tangle())
		if e != nil {
			return nil, e
		}
		return message, nil
	}

	// block for a certain amount of time until we know that the transaction
	// actually got booked by this node itself
	// TODO: replace with an actual more reactive way
	msg, err = messagelayer.AwaitMessageToBeBooked(issueTransaction, tx.ID(), c.maxTxBookedAwaitTime)
	if err != nil {
		return nil, "", xerrors.Errorf("%w: tx %s", err, tx.ID().String())
	}

	c.addAddressToBlacklist(addr)

	return msg, tx.ID().String(), nil
}

// collectUTXOsForFunding iterates over the faucet's UTXOs until the token threshold is reached.
// this function also returns the remainder balance for the given outputs.
func (c *Component) collectUTXOsForFunding() (inputs ledgerstate.Inputs, addrsIndices map[uint64]ledgerstate.Inputs, remainder int64) {
	var total = c.tokensPerRequest
	var i uint64
	addrsIndices = map[uint64]ledgerstate.Inputs{}

	// get a list of address for inputs
	for i = 0; total > 0; i++ {
		addr := c.seed.Address(i).Address()
		cachedOutputs := messagelayer.Tangle().LedgerState.OutputsOnAddress(addr)
		cachedOutputs.Consume(func(output ledgerstate.Output) {
			messagelayer.Tangle().LedgerState.OutputMetadata(output.ID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
				if outputMetadata.ConsumerCount() > 0 || total == 0 {
					return
				}
				var val int64
				output.Balances().ForEach(func(_ ledgerstate.Color, balance uint64) bool {
					val += int64(balance)
					return true
				})
				addrsIndices[i] = append(addrsIndices[i], ledgerstate.NewUTXOInput(output.ID()))

				// get unspent output ids and check if it's conflict
				if val <= total {
					total -= val
				} else {
					remainder = val - total
					total = 0
				}
				inputs = append(inputs, ledgerstate.NewUTXOInput(output.ID()))
			})
		})
	}

	return
}

// nextUnusedAddress generates an unused address from the faucet seed.
func (c *Component) nextUnusedAddress() ledgerstate.Address {
	var index uint64
	for index = 0; ; index++ {
		addr := c.seed.Address(index).Address()
		cachedOutputs := messagelayer.Tangle().LedgerState.OutputsOnAddress(addr)
		cachedOutputs.Release()
		if len(cachedOutputs.Unwrap()) == 0 {
			return addr
		}
	}
}

type wallet struct {
	keyPair ed25519.KeyPair
	address *ledgerstate.ED25519Address
}

func (w wallet) privateKey() ed25519.PrivateKey {
	return w.keyPair.PrivateKey
}

func (w wallet) publicKey() ed25519.PublicKey {
	return w.keyPair.PublicKey
}
func (w wallet) sign(txEssence *ledgerstate.TransactionEssence) *ledgerstate.ED25519Signature {
	return ledgerstate.NewED25519Signature(w.publicKey(), ed25519.Signature(w.privateKey().Sign(txEssence.Bytes())))
}
