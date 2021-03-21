package tangle

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/types"
)

// region MessageTestFramework /////////////////////////////////////////////////////////////////////////////////////////

// MessageTestFramework implements a framework for conveniently issuing messages in a tangle as part of unit tests in a
// simplified way.
type MessageTestFramework struct {
	tangle           *Tangle
	messagesByAlias  map[string]*Message
	walletsByAlias   map[string]wallet
	walletsByAddress map[ledgerstate.Address]wallet
	inputsByAlias    map[string]ledgerstate.Input
	outputsByAlias   map[string]ledgerstate.Output
	outputsByID      map[ledgerstate.OutputID]ledgerstate.Output
	options          *MessageTestFrameworkOptions
	messagesBookedWG sync.WaitGroup
}

// NewMessageTestFramework is the constructor of the MessageTestFramework.
func NewMessageTestFramework(tangle *Tangle, options ...MessageTestFrameworkOption) (messageTestFramework *MessageTestFramework) {
	messageTestFramework = &MessageTestFramework{
		tangle:           tangle,
		messagesByAlias:  make(map[string]*Message),
		walletsByAlias:   make(map[string]wallet),
		walletsByAddress: make(map[ledgerstate.Address]wallet),
		inputsByAlias:    make(map[string]ledgerstate.Input),
		outputsByAlias:   make(map[string]ledgerstate.Output),
		outputsByID:      make(map[ledgerstate.OutputID]ledgerstate.Output),
		options:          NewMessageTestFrameworkOptions(options...),
	}

	messageTestFramework.createGenesisOutputs()

	tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(func(messageID MessageID) {
		messageTestFramework.messagesBookedWG.Done()
	}))
	tangle.Events.MessageInvalid.Attach(events.NewClosure(func(messageID MessageID) {
		messageTestFramework.messagesBookedWG.Done()
	}))

	return
}

// CreateMessage creates a Message with the given alias and MessageTestFrameworkMessageOptions.
func (m *MessageTestFramework) CreateMessage(messageAlias string, messageOptions ...MessageOption) (message *Message) {
	options := NewMessageTestFrameworkMessageOptions(messageOptions...)

	if transaction := m.buildTransaction(options); transaction != nil {
		m.messagesByAlias[messageAlias] = newTestParentsPayloadMessage(transaction, m.strongParentIDs(options), m.weakParentIDs(options))
		return m.messagesByAlias[messageAlias]
	}

	m.messagesByAlias[messageAlias] = newTestParentsDataMessage(messageAlias, m.strongParentIDs(options), m.weakParentIDs(options))
	return m.messagesByAlias[messageAlias]
}

// IssueMessages stores the given Messages in the Storage and triggers the processing by the Tangle.
func (m *MessageTestFramework) IssueMessages(messageAliases ...string) *MessageTestFramework {
	m.messagesBookedWG.Add(len(messageAliases))

	for _, messageAlias := range messageAliases {
		m.tangle.Storage.StoreMessage(m.messagesByAlias[messageAlias])
	}

	return m
}

// WaitMessagesBooked waits for all Messages to be processed by the Booker.
func (m *MessageTestFramework) WaitMessagesBooked() {
	m.messagesBookedWG.Wait()
}

// Message retrieves the Messages that is associated with the given alias.
func (m *MessageTestFramework) Message(alias string) (message *Message) {
	return m.messagesByAlias[alias]
}

// createGenesisOutputs initializes the Outputs that are used by the MessageTestFramework as the genesis.
func (m *MessageTestFramework) createGenesisOutputs() {
	genesisOutputs := make(map[ledgerstate.Address]*ledgerstate.ColoredBalances)

	for alias, balance := range m.options.genesisOutputs {
		addressWallet := createWallets(1)[0]

		m.walletsByAlias[alias] = addressWallet
		m.walletsByAddress[addressWallet.address] = addressWallet

		genesisOutputs[addressWallet.address] = ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: balance,
		})
	}

	for alias, coloredBalances := range m.options.coloredGenesisOutputs {
		wallet := createWallets(1)[0]
		m.walletsByAlias[alias] = wallet
		m.walletsByAddress[wallet.address] = wallet

		genesisOutputs[wallet.address] = ledgerstate.NewColoredBalances(coloredBalances)
	}

	m.tangle.LedgerState.LoadSnapshot(map[ledgerstate.TransactionID]map[ledgerstate.Address]*ledgerstate.ColoredBalances{
		ledgerstate.GenesisTransactionID: genesisOutputs,
	})

	for alias := range m.options.genesisOutputs {
		m.tangle.LedgerState.utxoDAG.AddressOutputMapping(m.walletsByAlias[alias].address).Consume(func(addressOutputMapping *ledgerstate.AddressOutputMapping) {
			m.tangle.LedgerState.utxoDAG.Output(addressOutputMapping.OutputID()).Consume(func(output ledgerstate.Output) {
				m.outputsByAlias[alias] = output
				m.outputsByID[addressOutputMapping.OutputID()] = output
				m.inputsByAlias[alias] = ledgerstate.NewUTXOInput(addressOutputMapping.OutputID())
			})
		})
	}

	for alias := range m.options.coloredGenesisOutputs {
		m.tangle.LedgerState.utxoDAG.AddressOutputMapping(m.walletsByAlias[alias].address).Consume(func(addressOutputMapping *ledgerstate.AddressOutputMapping) {
			m.tangle.LedgerState.utxoDAG.Output(addressOutputMapping.OutputID()).Consume(func(output ledgerstate.Output) {
				m.outputsByAlias[alias] = output
				m.outputsByID[addressOutputMapping.OutputID()] = output
			})
		})
	}
}

// buildTransaction creates a Transaction from the given MessageTestFrameworkMessageOptions. It returns nil if there are
// no Transaction related MessageTestFrameworkMessageOptions.
func (m *MessageTestFramework) buildTransaction(options *MessageTestFrameworkMessageOptions) (transaction *ledgerstate.Transaction) {
	if len(options.inputs) == 0 || len(options.outputs) == 0 {
		return
	}

	inputs := make([]ledgerstate.Input, 0)
	for inputAlias := range options.inputs {
		inputs = append(inputs, m.inputsByAlias[inputAlias])
	}

	outputs := make([]ledgerstate.Output, 0)
	for alias, balance := range options.outputs {
		wallet := createWallets(1)[0]
		m.walletsByAlias[alias] = wallet
		m.walletsByAddress[wallet.address] = wallet

		m.outputsByAlias[alias] = ledgerstate.NewSigLockedSingleOutput(balance, m.walletsByAlias[alias].address)

		outputs = append(outputs, m.outputsByAlias[alias])
	}
	for alias, balances := range options.coloredOutputs {
		wallet := createWallets(1)[0]
		m.walletsByAlias[alias] = wallet
		m.walletsByAddress[wallet.address] = wallet

		m.outputsByAlias[alias] = ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(balances), m.walletsByAlias[alias].address)

		outputs = append(outputs, m.outputsByAlias[alias])
	}

	transaction = makeTransaction(ledgerstate.NewInputs(inputs...), ledgerstate.NewOutputs(outputs...), m.outputsByID, m.walletsByAddress)
	for outputIndex, output := range transaction.Essence().Outputs() {
		for alias, aliasedOutput := range m.outputsByAlias {
			if aliasedOutput == output {
				output.SetID(ledgerstate.NewOutputID(transaction.ID(), uint16(outputIndex)))

				m.outputsByID[output.ID()] = output
				m.inputsByAlias[alias] = ledgerstate.NewUTXOInput(output.ID())

				break
			}
		}
	}

	return
}

// strongParentIDs returns the MessageIDs that were defined to be the strong parents of the
// MessageTestFrameworkMessageOptions.
func (m *MessageTestFramework) strongParentIDs(options *MessageTestFrameworkMessageOptions) (strongParentIDs MessageIDs) {
	strongParentIDs = make(MessageIDs, 0)
	for strongParentAlias := range options.strongParents {
		if strongParentAlias == "Genesis" {
			strongParentIDs = append(strongParentIDs, EmptyMessageID)

			continue
		}

		strongParentIDs = append(strongParentIDs, m.messagesByAlias[strongParentAlias].ID())
	}

	return
}

// weakParentIDs returns the MessageIDs that were defined to be the weak parents of the
// MessageTestFrameworkMessageOptions.
func (m *MessageTestFramework) weakParentIDs(options *MessageTestFrameworkMessageOptions) (weakParentIDs MessageIDs) {
	weakParentIDs = make(MessageIDs, 0)
	for weakParentAlias := range options.strongParents {
		if weakParentAlias == "Genesis" {
			weakParentIDs = append(weakParentIDs, EmptyMessageID)

			continue
		}

		weakParentIDs = append(weakParentIDs, m.messagesByAlias[weakParentAlias].ID())
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MessageTestFrameworkOptions //////////////////////////////////////////////////////////////////////////////////

// MessageTestFrameworkOptions is a container that holds the values of all configurable options of the
// MessageTestFramework.
type MessageTestFrameworkOptions struct {
	genesisOutputs        map[string]uint64
	coloredGenesisOutputs map[string]map[ledgerstate.Color]uint64
}

// NewMessageTestFrameworkOptions is the constructor for the MessageTestFrameworkOptions.
func NewMessageTestFrameworkOptions(options ...MessageTestFrameworkOption) (frameworkOptions *MessageTestFrameworkOptions) {
	frameworkOptions = &MessageTestFrameworkOptions{
		genesisOutputs:        make(map[string]uint64),
		coloredGenesisOutputs: make(map[string]map[ledgerstate.Color]uint64),
	}

	for _, option := range options {
		option(frameworkOptions)
	}

	return
}

// MessageTestFrameworkOption is the type that is used for options that can be passed into the MessageTestFramework to
// configure its behavior.
type MessageTestFrameworkOption func(*MessageTestFrameworkOptions)

// WithGenesisOutput returns a MessageTestFrameworkOption that defines a genesis Output that is loaded as part of the
// initial snapshot.
func WithGenesisOutput(alias string, balance uint64) MessageTestFrameworkOption {
	return func(options *MessageTestFrameworkOptions) {
		if _, exists := options.genesisOutputs[alias]; exists {
			panic(fmt.Sprintf("duplicate genesis output alias (%s)", alias))
		}
		if _, exists := options.coloredGenesisOutputs[alias]; exists {
			panic(fmt.Sprintf("duplicate genesis output alias (%s)", alias))
		}

		options.genesisOutputs[alias] = balance
	}
}

// WithColoredGenesisOutput returns a MessageTestFrameworkOption that defines a genesis Output that is loaded as part of
// the initial snapshot and that supports colored coins.
func WithColoredGenesisOutput(alias string, balances map[ledgerstate.Color]uint64) MessageTestFrameworkOption {
	return func(options *MessageTestFrameworkOptions) {
		if _, exists := options.genesisOutputs[alias]; exists {
			panic(fmt.Sprintf("duplicate genesis output alias (%s)", alias))
		}
		if _, exists := options.coloredGenesisOutputs[alias]; exists {
			panic(fmt.Sprintf("duplicate genesis output alias (%s)", alias))
		}

		options.coloredGenesisOutputs[alias] = balances
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MessageTestFrameworkMessageOptions ///////////////////////////////////////////////////////////////////////////

// MessageTestFrameworkMessageOptions is a struct that represents a collection of options that can be set when creating
// a Message with the MessageTestFramework.
type MessageTestFrameworkMessageOptions struct {
	inputs         map[string]types.Empty
	outputs        map[string]uint64
	coloredOutputs map[string]map[ledgerstate.Color]uint64
	strongParents  map[string]types.Empty
	weakParents    map[string]types.Empty
}

func NewMessageTestFrameworkMessageOptions(options ...MessageOption) (messageOptions *MessageTestFrameworkMessageOptions) {
	messageOptions = &MessageTestFrameworkMessageOptions{
		inputs:        make(map[string]types.Empty),
		outputs:       make(map[string]uint64),
		strongParents: make(map[string]types.Empty),
		weakParents:   make(map[string]types.Empty),
	}

	for _, option := range options {
		option(messageOptions)
	}

	return
}

type MessageOption func(*MessageTestFrameworkMessageOptions)

// WithInputs returns a MessageOption that is used to provide the Inputs of the Transaction.
func WithInputs(inputAliases ...string) MessageOption {
	return func(options *MessageTestFrameworkMessageOptions) {
		for _, inputAlias := range inputAliases {
			options.inputs[inputAlias] = types.Void
		}
	}
}

func WithOutput(alias string, balance uint64) MessageOption {
	return func(options *MessageTestFrameworkMessageOptions) {
		options.outputs[alias] = balance
	}
}

func WithColoredOutput(alias string, balances map[ledgerstate.Color]uint64) MessageOption {
	return func(options *MessageTestFrameworkMessageOptions) {
		options.coloredOutputs[alias] = balances
	}
}

func WithStrongParents(messageAliases ...string) MessageOption {
	return func(options *MessageTestFrameworkMessageOptions) {
		for _, messageAlias := range messageAliases {
			options.strongParents[messageAlias] = types.Void
		}
	}
}

func WithWeakParents(messageAliases ...string) MessageOption {
	return func(options *MessageTestFrameworkMessageOptions) {
		for _, messageAlias := range messageAliases {
			options.weakParents[messageAlias] = types.Void
		}
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Utility functions ////////////////////////////////////////////////////////////////////////////////////////////

var sequenceNumber uint64

func nextSequenceNumber() uint64 {
	return atomic.AddUint64(&sequenceNumber, 1) - 1
}

func newTestNonceMessage(nonce uint64) *Message {
	return NewMessage([]MessageID{EmptyMessageID}, []MessageID{}, time.Time{}, ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte("test")), nonce, ed25519.Signature{})
}

func newTestDataMessage(payloadString string) *Message {
	return NewMessage([]MessageID{EmptyMessageID}, []MessageID{}, time.Now(), ed25519.PublicKey{}, nextSequenceNumber(), payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{})
}

func newTestParentsDataMessage(payloadString string, strongParents, weakParents []MessageID) *Message {
	return NewMessage(strongParents, weakParents, time.Now(), ed25519.PublicKey{}, nextSequenceNumber(), payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{})
}

func newTestParentsDataWithTimestamp(payloadString string, strongParents, weakParents []MessageID, timestamp time.Time) *Message {
	return NewMessage(strongParents, weakParents, timestamp, ed25519.PublicKey{}, nextSequenceNumber(), payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{})
}

func newTestParentsPayloadMessage(payload payload.Payload, strongParents, weakParents []MessageID) *Message {
	return NewMessage(strongParents, weakParents, time.Now(), ed25519.PublicKey{}, nextSequenceNumber(), payload, 0, ed25519.Signature{})
}

func newTestParentsPayloadWithTimestamp(payload payload.Payload, strongParents, weakParents []MessageID, timestamp time.Time) *Message {
	return NewMessage(strongParents, weakParents, timestamp, ed25519.PublicKey{}, nextSequenceNumber(), payload, 0, ed25519.Signature{})
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

func createWallets(n int) []wallet {
	wallets := make([]wallet, n)
	for i := 0; i < n; i++ {
		kp := ed25519.GenerateKeyPair()
		wallets[i] = wallet{
			kp,
			ledgerstate.NewED25519Address(kp.PublicKey),
		}
	}
	return wallets
}

func (w wallet) sign(txEssence *ledgerstate.TransactionEssence) *ledgerstate.ED25519Signature {
	return ledgerstate.NewED25519Signature(w.publicKey(), ed25519.Signature(w.privateKey().Sign(txEssence.Bytes())))
}

// addressFromInput retrieves the Address belonging to an Input by looking it up in the outputs that we have created for
// the tests.
func addressFromInput(input ledgerstate.Input, outputsByID ledgerstate.OutputsByID) ledgerstate.Address {
	typeCastedInput, ok := input.(*ledgerstate.UTXOInput)
	if !ok {
		panic("unexpected Input type")
	}

	switch referencedOutput := outputsByID[typeCastedInput.ReferencedOutputID()]; referencedOutput.Type() {
	case ledgerstate.SigLockedSingleOutputType:
		typeCastedOutput, ok := referencedOutput.(*ledgerstate.SigLockedSingleOutput)
		if !ok {
			panic("failed to type cast SigLockedSingleOutput")
		}

		return typeCastedOutput.Address()
	case ledgerstate.SigLockedColoredOutputType:
		typeCastedOutput, ok := referencedOutput.(*ledgerstate.SigLockedColoredOutput)
		if !ok {
			panic("failed to type cast SigLockedColoredOutput")
		}
		return typeCastedOutput.Address()
	default:
		panic("unexpected Output type")
	}
}

func messageBranchID(tangle *Tangle, messageID MessageID) (branchID ledgerstate.BranchID, err error) {
	if !tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		branchID = messageMetadata.BranchID()
		// fmt.Println(messageID)
		// fmt.Println(messageMetadata.StructureDetails())
	}) {
		return branchID, fmt.Errorf("missing message metadata")
	}
	return
}

func transactionBranchID(tangle *Tangle, transactionID ledgerstate.TransactionID) (branchID ledgerstate.BranchID, err error) {
	if !tangle.LedgerState.TransactionMetadata(transactionID).Consume(func(metadata *ledgerstate.TransactionMetadata) {
		branchID = metadata.BranchID()
	}) {
		return branchID, fmt.Errorf("missing transaction metadata")
	}
	return
}

func makeTransaction(inputs ledgerstate.Inputs, outputs ledgerstate.Outputs, outputsByID map[ledgerstate.OutputID]ledgerstate.Output, walletsByAddress map[ledgerstate.Address]wallet, genesisWallet ...wallet) *ledgerstate.Transaction {
	txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, inputs, outputs)
	unlockBlocks := make([]ledgerstate.UnlockBlock, len(txEssence.Inputs()))
	for i, input := range txEssence.Inputs() {
		w := wallet{}
		if genesisWallet != nil {
			w = genesisWallet[0]
		} else {
			w = walletsByAddress[addressFromInput(input, outputsByID)]
		}
		unlockBlocks[i] = ledgerstate.NewSignatureUnlockBlock(w.sign(txEssence))
	}
	return ledgerstate.NewTransaction(txEssence, unlockBlocks)
}

func selectIndex(transaction *ledgerstate.Transaction, w wallet) (index uint16) {
	for i, output := range transaction.Essence().Outputs() {
		if w.address == output.(*ledgerstate.SigLockedSingleOutput).Address() {
			return uint16(i)
		}
	}
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
