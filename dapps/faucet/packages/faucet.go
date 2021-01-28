package faucet

import (
	"errors"
	"fmt"
	"sync"
	"time"

	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	faucetpayload "github.com/iotaledger/goshimmer/dapps/faucet/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address/signaturescheme"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	"github.com/iotaledger/hive.go/datastructure/orderedmap"
	"github.com/iotaledger/hive.go/identity"
)

var (
	// ErrFundingTxNotBookedInTime is returned when a funding transaction didn't get booked
	// by this node in the maximum defined await time for it to get booked.
	ErrFundingTxNotBookedInTime = errors.New("funding transaction didn't get booked in time")
	// ErrAddressIsBlacklisted is returned if a funding can't be processed since the address is blacklisted.
	ErrAddressIsBlacklisted = errors.New("can't fund address as it is blacklisted")
	// ErrPrepareFaucet is returned if the faucet cannot prepare outputs.
	ErrPrepareFaucet = errors.New("can't prepare faucet outputs")
)

// New creates a new faucet using the given seed and tokensPerRequest config.
func New(seed []byte, tokensPerRequest int64, blacklistCapacity int, maxTxBookedAwaitTime time.Duration, preparedOutputsCount int) *Faucet {
	return &Faucet{
		tokensPerRequest:     tokensPerRequest,
		seed:                 walletseed.NewSeed(seed),
		maxTxBookedAwaitTime: maxTxBookedAwaitTime,
		blacklist:            orderedmap.New(),
		blacklistCapacity:    blacklistCapacity,
		preparedOutputsCount: preparedOutputsCount,
	}
}

// The Faucet implements a component which will send tokens to actors requesting tokens.
type Faucet struct {
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
	preparedOutputsCount int
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
	nodeID := identity.NewID(msg.IssuerPublicKey())

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
		nodeID,
	)

	// add remainder address if needed
	if remainder > 0 {
		remainAddr := f.nextUnusedAddress()
		tx.Outputs().Add(remainAddr, []*balance.Balance{balance.New(balance.ColorIOTA, remainder)})
	}

	for index := range addrsIndices {
		tx.Sign(signaturescheme.ED25519(*f.seed.KeyPair(index)))
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

	// last used address index
	lastUsedAddressIndex := uint64(0)
	for k := range addrsIndices {
		if k > lastUsedAddressIndex {
			lastUsedAddressIndex = k
		}
	}

	// block for a certain amount of time until we know that the transaction
	// actually got booked by this node itself
	// TODO: replace with an actual more reactive way
	if err := valuetransfers.AwaitTransactionToBeBooked(tx.ID(), f.maxTxBookedAwaitTime); err != nil {
		return nil, "", fmt.Errorf("%w: tx %s", err, tx.ID().String())
	}

	_, err = f.prepareMoreOutputs(lastUsedAddressIndex)
	if err != nil {
		err = fmt.Errorf("%w: %s", ErrPrepareFaucet, err.Error())
	}
	f.addAddressToBlacklist(addr)
	return msg, tx.ID().String(), err
}

// PrepareGenesisOutput splits genesis output to CfgFaucetPreparedOutputsCount number of outputs.
// If this process has been done before, it'll not do it again.
func (f *Faucet) PrepareGenesisOutput() (msg *message.Message, err error) {
	// get total funds
	firstAddr := f.seed.Address(0).Address
	var faucetTotal int64
	valuetransfers.Tangle().TransactionOutput(transaction.NewOutputID(firstAddr, transaction.GenesisID)).Consume(func(output *tangle.Output) {
		// should only be done once
		if output.ConsumerCount() > 0 {
			return
		}

		// gather all balance
		for _, coloredBalance := range output.Balances() {
			faucetTotal += coloredBalance.Value
		}

		msg, err = f.splitOutput(output.ID(), 0, faucetTotal)
	})
	return
}

// collectUTXOsForFunding iterates over the faucet's UTXOs until the token threshold is reached.
// this function also returns the remainder balance for the given outputs.
func (f *Faucet) collectUTXOsForFunding() (outputIds []transaction.OutputID, addrsIndices map[uint64]struct{}, remainder int64) {
	var total = f.tokensPerRequest
	var i uint64
	addrsIndices = map[uint64]struct{}{}

	// get a list of address for inputs
	for i = 0; total > 0; i++ {
		addr := f.seed.Address(i).Address
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
func (f *Faucet) nextUnusedAddress(startIndex ...uint64) address.Address {
	var index uint64
	if len(startIndex) > 0 {
		index = startIndex[0]
	}
	for ; ; index++ {
		addr := f.seed.Address(index).Address
		cachedOutputs := valuetransfers.Tangle().OutputsOnAddress(addr)
		if len(cachedOutputs) == 0 {
			// unused address
			cachedOutputs.Release()
			return addr
		}
		cachedOutputs.Release()
	}
}

// prepareMoreOutputs prepares more outputs on the faucet if most of the already prepared outputs have been consumed.
func (f *Faucet) prepareMoreOutputs(lastUsedAddressIndex uint64) (msg *message.Message, err error) {
	lastOne := int64(f.preparedOutputsCount) - (int64(lastUsedAddressIndex) % int64(f.preparedOutputsCount))
	if lastOne != int64(f.preparedOutputsCount) {
		return
	}
	var remainderOutputID transaction.OutputID
	found := false
	var remainder int64
	var remainderAddressIndex uint64
	// find remainder output
	for i := lastUsedAddressIndex + 1; !found; i++ {
		addr := f.seed.Address(i).Address
		valuetransfers.Tangle().OutputsOnAddress(addr).Consume(func(output *tangle.Output) {
			if output.ConsumerCount() > 0 {
				return
			}
			var val int64
			for _, coloredBalance := range output.Balances() {
				val += coloredBalance.Value
			}
			// not a prepared output.
			if val != f.tokensPerRequest {
				remainderOutputID = output.ID()
				remainderAddressIndex = i
				found = true
				remainder = val
			}
		})
	}

	return f.splitOutput(remainderOutputID, remainderAddressIndex, remainder)
}

// splitOutput splits the remainder into `f.preparedOutputsCount` outputs.
func (f *Faucet) splitOutput(remainderOutputID transaction.OutputID, remainderAddressIndex uint64, remainder int64) (msg *message.Message, err error) {
	if remainder < f.tokensPerRequest {
		return
	}
	var totalPrepared int64
	outputs := make(map[address.Address][]*balance.Balance)
	addrIndex := remainderAddressIndex + 1
	for i := 0; i < f.preparedOutputsCount; i++ {
		if totalPrepared+f.tokensPerRequest > remainder {
			break
		}
		nextAddr := f.nextUnusedAddress(addrIndex)
		addrIndex++
		outputs[nextAddr] = []*balance.Balance{balance.New(balance.ColorIOTA, f.tokensPerRequest)}
		totalPrepared += f.tokensPerRequest
	}

	faucetBalance := remainder - totalPrepared
	if faucetBalance > 0 {
		nextAddr := f.nextUnusedAddress(addrIndex)
		outputs[nextAddr] = []*balance.Balance{balance.New(balance.ColorIOTA, faucetBalance)}
	}
	tx := transaction.New(
		transaction.NewInputs(remainderOutputID),
		transaction.NewOutputs(outputs),
		local.GetInstance().ID(),
	)
	tx.Sign(signaturescheme.ED25519(*f.seed.KeyPair(remainderAddressIndex)))
	payload, err := valuetransfers.ValueObjectFactory().IssueTransaction(tx)
	if err != nil {
		return
	}

	msg, err = issuer.IssuePayload(payload)
	if err != nil {
		return
	}
	return
}
