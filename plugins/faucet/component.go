package faucet

import (
	"sync"
	"time"

	"golang.org/x/xerrors"

	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address/signaturescheme"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	valuetangle "github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/datastructure/orderedmap"
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
func (c *Component) IsAddressBlacklisted(addr address.Address) bool {
	_, blacklisted := c.blacklist.Get(addr)
	return blacklisted
}

// adds the given address to the blacklist and removes the oldest blacklist entry
// if it would go over capacity.
func (c *Component) addAddressToBlacklist(addr address.Address) {
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

	// get the output ids for the inputs and remainder balance
	outputIds, addrsIndices, remainder := c.collectUTXOsForFunding()

	tx := transaction.New(
		// inputs
		transaction.NewInputs(outputIds...),

		// outputs
		transaction.NewOutputs(map[address.Address][]*balance.Balance{
			addr: {
				balance.New(balance.ColorIOTA, c.tokensPerRequest),
			},
		}),
	)

	// add remainder address if needed
	if remainder > 0 {
		remainAddr := c.nextUnusedAddress()
		tx.Outputs().Add(remainAddr, []*balance.Balance{balance.New(balance.ColorIOTA, remainder)})
	}

	for index := range addrsIndices {
		tx.Sign(signaturescheme.ED25519(*c.seed.KeyPair(index)))
	}

	// prepare value payload with value factory
	payload, err := valuetransfers.ValueObjectFactory().IssueTransaction(tx)
	if err != nil {
		return nil, "", xerrors.Errorf("failed to issue transaction: %w", err)
	}

	// attach to message layer
	issueTransaction := func() (*tangle.Message, error) {
		message, e := issuer.IssuePayload(payload, messagelayer.Tangle())
		if e != nil {
			return nil, e
		}
		return message, nil
	}

	// block for a certain amount of time until we know that the transaction
	// actually got booked by this node itself
	// TODO: replace with an actual more reactive way
	msg, err = valuetransfers.AwaitTransactionToBeBooked(issueTransaction, tx.ID(), c.maxTxBookedAwaitTime)
	if err != nil {
		return nil, "", xerrors.Errorf("%w: tx %s", err, tx.ID().String())
	}

	c.addAddressToBlacklist(addr)

	return msg, tx.ID().String(), nil
}

// collectUTXOsForFunding iterates over the faucet's UTXOs until the token threshold is reached.
// this function also returns the remainder balance for the given outputs.
func (c *Component) collectUTXOsForFunding() (outputIds []transaction.OutputID, addrsIndices map[uint64]struct{}, remainder int64) {
	var total = c.tokensPerRequest
	var i uint64
	addrsIndices = map[uint64]struct{}{}

	// get a list of address for inputs
	for i = 0; total > 0; i++ {
		addr := c.seed.Address(i).Address
		valuetransfers.Tangle().OutputsOnAddress(addr).Consume(func(output *valuetangle.Output) {
			if output.ConsumerCount() > 0 || total == 0 {
				return
			}

			var val int64
			for _, coloredBalance := range output.Balances() {
				val += coloredBalance.Value
			}
			addrsIndices[i] = struct{}{}

			// get unspent output ids and check if it's conflict
			if val <= total {
				total -= val
			} else {
				remainder = val - total
				total = 0
			}
			outputIds = append(outputIds, output.ID())
		})
	}

	return
}

// nextUnusedAddress generates an unused address from the faucet seed.
func (c *Component) nextUnusedAddress() address.Address {
	var index uint64
	for index = 0; ; index++ {
		addr := c.seed.Address(index).Address
		cachedOutputs := valuetransfers.Tangle().OutputsOnAddress(addr)
		if len(cachedOutputs) == 0 {
			// unused address
			cachedOutputs.Release()
			return addr
		}
		cachedOutputs.Release()
	}
}
