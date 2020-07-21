package faucet

import (
	"errors"
	"fmt"
	"sync"
	"time"

	faucetpayload "github.com/iotaledger/goshimmer/dapps/faucet/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address/signaturescheme"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/wallet"
	"github.com/iotaledger/goshimmer/packages/binary/datastructure/orderedmap"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/plugins/issuer"
)

var (
	// ErrFundingTxNotBookedInTime is returned when a funding transaction didn't get booked
	// by this node in the maximum defined await time for it to get booked.
	ErrFundingTxNotBookedInTime = errors.New("funding transaction didn't get booked in time")
	// ErrAddressIsBlacklisted is returned if a funding can't be processed since the address is blacklisted.
	ErrAddressIsBlacklisted = errors.New("can't fund address as it is blacklisted")
)

// New creates a new faucet using the given seed and tokensPerRequest config.
func New(seed []byte, tokensPerRequest int64, blacklistCapacity int, maxTxBookedAwaitTime time.Duration) *Faucet {
	return &Faucet{
		tokensPerRequest:     tokensPerRequest,
		wallet:               wallet.New(seed),
		maxTxBookedAwaitTime: maxTxBookedAwaitTime,
		blacklist:            orderedmap.New(),
		blacklistCapacity:    blacklistCapacity,
	}
}

// The Faucet implements a component which will send tokens to actors requesting tokens.
type Faucet struct {
	sync.Mutex
	// the amount of tokens to send to every request
	tokensPerRequest int64
	// the wallet instance of the faucet holding the tokens
	wallet *wallet.Wallet
	// the time to await for the transaction fulfilling a funding request
	// to become booked in the value layer
	maxTxBookedAwaitTime time.Duration
	blacklistCapacity    int
	blacklist            *orderedmap.OrderedMap
}

// IsAddressBlacklisted checks whether the given address is currently blacklisted.
func (f *Faucet) IsAddressBlacklisted(addr address.Address) bool {
	_, blacklisted := f.blacklist.Get(addr)
	return blacklisted
}

// adds the given address to the blacklist and removes the oldest blacklist entry
// if it would go over capacity.
func (f *Faucet) addAddressToBlacklist(addr address.Address) {
	f.blacklist.Set(addr, true)
	if f.blacklist.Size() > f.blacklistCapacity {
		var headKey interface{}
		f.blacklist.ForEach(func(key, value interface{}) bool {
			headKey = key
			return false
		})
		f.blacklist.Delete(headKey)
	}
}

// SendFunds sends IOTA tokens to the address from faucet request.
func (f *Faucet) SendFunds(msg *message.Message) (m *message.Message, txID string, err error) {
	// ensure that only one request is being processed any given time
	f.Lock()
	defer f.Unlock()

	addr := msg.Payload().(*faucetpayload.Payload).Address()

	if f.IsAddressBlacklisted(addr) {
		return nil, "", ErrAddressIsBlacklisted
	}

	// get the output ids for the inputs and remainder balance
	outputIds, addrsIndices, remainder := f.collectUTXOsForFunding()

	tx := transaction.New(
		// inputs
		transaction.NewInputs(outputIds...),

		// outputs
		transaction.NewOutputs(map[address.Address][]*balance.Balance{
			addr: {
				balance.New(balance.ColorIOTA, f.tokensPerRequest),
			},
		}),
	)

	// add remainder address if needed
	if remainder > 0 {
		remainAddr := f.nextUnusedAddress()
		tx.Outputs().Add(remainAddr, []*balance.Balance{balance.New(balance.ColorIOTA, remainder)})
	}

	for index := range addrsIndices {
		tx.Sign(signaturescheme.ED25519(*f.wallet.Seed().KeyPair(index)))
	}

	// prepare value payload with value factory
	payload, err := valuetransfers.ValueObjectFactory().IssueTransaction(tx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to issue transaction: %w", err)
	}

	// attach to message layer
	msg, err = issuer.IssuePayload(payload)
	if err != nil {
		return nil, "", err
	}

	// block for a certain amount of time until we know that the transaction
	// actually got booked by this node itself
	// TODO: replace with an actual more reactive way
	if err := valuetransfers.AwaitTransactionToBeBooked(tx.ID(), f.maxTxBookedAwaitTime); err != nil {
		return nil, "", fmt.Errorf("%w: tx %s", err, tx.ID().String())
	}

	f.addAddressToBlacklist(addr)

	return msg, tx.ID().String(), nil
}

// collectUTXOsForFunding iterates over the faucet's UTXOs until the token threshold is reached.
// this function also returns the remainder balance for the given outputs.
func (f *Faucet) collectUTXOsForFunding() (outputIds []transaction.OutputID, addrsIndices map[uint64]struct{}, remainder int64) {
	var total = f.tokensPerRequest
	var i uint64
	addrsIndices = map[uint64]struct{}{}

	// get a list of address for inputs
	for i = 0; total > 0; i++ {
		addr := f.wallet.Seed().Address(i)
		valuetransfers.Tangle().OutputsOnAddress(addr).Consume(func(output *tangle.Output) {
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
func (f *Faucet) nextUnusedAddress() address.Address {
	var index uint64
	for index = 0; ; index++ {
		addr := f.wallet.Seed().Address(index)
		cachedOutputs := valuetransfers.Tangle().OutputsOnAddress(addr)
		if len(cachedOutputs) == 0 {
			// unused address
			cachedOutputs.Release()
			return addr
		}
		cachedOutputs.Release()
	}
}
