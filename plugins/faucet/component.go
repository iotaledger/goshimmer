package faucet

import (
	"sync"
	"time"

	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/datastructure/orderedmap"
	"github.com/iotaledger/hive.go/identity"
	"golang.org/x/xerrors"
)

// New creates a new faucet component using the given seed and tokensPerRequest config.
func New(seed []byte, tokensPerRequest int64, blacklistCapacity int, maxTxBookedAwaitTime time.Duration, preparedOutputsCount int) *Component {
	return &Component{
		tokensPerRequest:     tokensPerRequest,
		seed:                 walletseed.NewSeed(seed),
		maxTxBookedAwaitTime: maxTxBookedAwaitTime,
		blacklist:            orderedmap.New(),
		blacklistCapacity:    blacklistCapacity,
		preparedOutputsCount: preparedOutputsCount,
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
	preparedOutputsCount int
}

// IsAddressBlacklisted checks whether the given address is currently blacklisted.
func (c *Component) IsAddressBlacklisted(addr ledgerstate.Address) bool {
	_, blacklisted := c.blacklist.Get(addr.Base58())
	return blacklisted
}

// adds the given address to the blacklist and removes the oldest blacklist entry
// if it would go over capacity.
func (c *Component) addAddressToBlacklist(addr ledgerstate.Address) {
	c.blacklist.Set(addr.Base58(), true)
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

	nodeID := identity.NewID(msg.IssuerPublicKey())
	txEssence := ledgerstate.NewTransactionEssence(0, clock.SyncedTime(), nodeID, nodeID, ledgerstate.NewInputs(inputs...), ledgerstate.NewOutputs(outputs...))

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
		message, e := messagelayer.Tangle().IssuePayload(tx)
		if e != nil {
			return nil, e
		}
		return message, nil
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
	msg, err = messagelayer.AwaitMessageToBeBooked(issueTransaction, tx.ID(), c.maxTxBookedAwaitTime)
	if err != nil {
		return nil, "", xerrors.Errorf("%w: tx %s", err, tx.ID().String())
	}

	_, err = c.prepareMoreOutputs(lastUsedAddressIndex)
	if err != nil {
		err = xerrors.Errorf("%w: %s", ErrPrepareFaucet, err.Error())
	}
	c.addAddressToBlacklist(addr)

	return msg, tx.ID().String(), err
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
func (c *Component) nextUnusedAddress(startIndex ...uint64) ledgerstate.Address {
	var index uint64
	if len(startIndex) > 0 {
		index = startIndex[0]
	}
	for ; ; index++ {
		addr := c.seed.Address(index).Address()
		cachedOutputs := messagelayer.Tangle().LedgerState.OutputsOnAddress(addr)
		cachedOutputs.Release()
		if len(cachedOutputs.Unwrap()) == 0 {
			return addr
		}
	}
}

// PrepareGenesisOutput splits genesis output to CfgFaucetPreparedOutputsCount number of outputs.
// If this process has been done before, it'll not do it again.
func (c *Component) PrepareGenesisOutput() (msg *tangle.Message, err error) {
	genesisOutputID := ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0)
	// get total funds
	var faucetTotal int64
	messagelayer.Tangle().LedgerState.Output(genesisOutputID).Consume(func(output ledgerstate.Output) {
		output.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
			faucetTotal += int64(balance)
			return true
		})
	})

	cachedOutputMeta := messagelayer.Tangle().LedgerState.OutputMetadata(genesisOutputID)
	defer cachedOutputMeta.Release()
	outputMetadata := cachedOutputMeta.Unwrap()
	if outputMetadata == nil {
		return nil, xerrors.Errorf("can't locate genesis output metadata: %s", genesisOutputID.Base58())
	}
	if outputMetadata.ConsumerCount() > 0 {
		log.Info("genesis output already spent")
		return nil, nil
	}

	msg, err = c.splitOutput(genesisOutputID, 0, faucetTotal)
	return
}

// prepareMoreOutputs prepares more outputs on the faucet if most of the already prepared outputs have been consumed.
func (c *Component) prepareMoreOutputs(lastUsedAddressIndex uint64) (msg *tangle.Message, err error) {
	lastOne := int64(c.preparedOutputsCount) - (int64(lastUsedAddressIndex) % int64(c.preparedOutputsCount))
	if lastOne != int64(c.preparedOutputsCount) {
		return
	}
	var remainderOutputID ledgerstate.OutputID
	found := false
	var remainder int64
	var remainderAddressIndex uint64
	// find remainder output
	for i := lastUsedAddressIndex + 1; !found; i++ {
		addr := c.seed.Address(i).Address()
		messagelayer.Tangle().LedgerState.OutputsOnAddress(addr).Consume(func(output ledgerstate.Output) {
			var consumerCount int
			messagelayer.Tangle().LedgerState.OutputMetadata(output.ID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
				consumerCount = outputMetadata.ConsumerCount()
			})
			if consumerCount > 0 {
				return
			}
			var val int64
			output.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
				val += int64(balance)
				return true
			})
			// not a prepared output.
			if val != c.tokensPerRequest {
				remainderOutputID = output.ID()
				remainderAddressIndex = i
				found = true
				remainder = val
			}
		})
	}

	return c.splitOutput(remainderOutputID, remainderAddressIndex, remainder)
}

// splitOutput splits the remainder into `f.preparedOutputsCount` outputs.
func (c *Component) splitOutput(remainderOutputID ledgerstate.OutputID, remainderAddressIndex uint64, remainder int64) (msg *tangle.Message, err error) {
	if remainder < c.tokensPerRequest {
		return
	}
	var totalPrepared int64
	outputs := ledgerstate.Outputs{}
	addrIndex := remainderAddressIndex + 1
	for i := 0; i < c.preparedOutputsCount; i++ {
		if totalPrepared+c.tokensPerRequest > remainder {
			break
		}
		nextAddr := c.nextUnusedAddress(addrIndex)
		addrIndex++
		outputs = append(outputs, ledgerstate.NewSigLockedColoredOutput(
			ledgerstate.NewColoredBalances(
				map[ledgerstate.Color]uint64{
					ledgerstate.ColorIOTA: uint64(c.tokensPerRequest),
				}),
			nextAddr,
		),
		)
		totalPrepared += c.tokensPerRequest
	}

	faucetBalance := remainder - totalPrepared
	if faucetBalance > 0 {
		nextAddr := c.nextUnusedAddress(addrIndex)
		outputs = append(outputs, ledgerstate.NewSigLockedColoredOutput(
			ledgerstate.NewColoredBalances(
				map[ledgerstate.Color]uint64{
					ledgerstate.ColorIOTA: uint64(faucetBalance),
				}),
			nextAddr,
		),
		)
	}
	inputs := ledgerstate.Inputs{
		ledgerstate.NewUTXOInput(remainderOutputID),
	}

	essence := ledgerstate.NewTransactionEssence(
		0,
		time.Now(),
		local.GetInstance().ID(),
		local.GetInstance().ID(),
		ledgerstate.NewInputs(inputs...),
		ledgerstate.NewOutputs(outputs...),
	)

	unlockBlocks := make([]ledgerstate.UnlockBlock, len(essence.Inputs()))
	inputIndex := 0
	w := wallet{keyPair: *c.seed.KeyPair(remainderAddressIndex)}
	for range inputs {
		unlockBlock := ledgerstate.NewSignatureUnlockBlock(w.sign(essence))
		unlockBlocks[inputIndex] = unlockBlock
		inputIndex++
	}

	// prepare the transaction with outputs
	tx := ledgerstate.NewTransaction(
		essence,
		unlockBlocks,
	)

	// attach to message layer
	issueTransaction := func() (*tangle.Message, error) {
		message, e := messagelayer.Tangle().IssuePayload(tx)
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
		return nil, xerrors.Errorf("%w: tx %s", err, tx.ID().String())
	}

	return
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
