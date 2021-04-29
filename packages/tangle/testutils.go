package tangle

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

// region MessageTestFramework /////////////////////////////////////////////////////////////////////////////////////////

// MessageTestFramework implements a framework for conveniently issuing messages in a tangle as part of unit tests in a
// simplified way.
type MessageTestFramework struct {
	tangle                   *Tangle
	messagesByAlias          map[string]*Message
	walletsByAlias           map[string]wallet
	walletsByAddress         map[ledgerstate.Address]wallet
	inputsByAlias            map[string]ledgerstate.Input
	outputsByAlias           map[string]ledgerstate.Output
	outputsByID              map[ledgerstate.OutputID]ledgerstate.Output
	options                  *MessageTestFrameworkOptions
	oldIncreaseIndexCallback markers.IncreaseIndexCallback
	messagesBookedWG         sync.WaitGroup
	approvalWeightProcessed  sync.WaitGroup
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

	tangle.Booker.Events.MessageBooked.AttachAfter(events.NewClosure(func(messageID MessageID) {
		messageTestFramework.messagesBookedWG.Done()
	}))
	tangle.ApprovalWeightManager.Events.MessageProcessed.AttachAfter(events.NewClosure(func(messageID MessageID) {
		messageTestFramework.approvalWeightProcessed.Done()
	}))
	tangle.Events.MessageInvalid.AttachAfter(events.NewClosure(func(messageID MessageID) {
		messageTestFramework.messagesBookedWG.Done()
		messageTestFramework.approvalWeightProcessed.Done()
	}))

	return
}

// CreateMessage creates a Message with the given alias and MessageTestFrameworkMessageOptions.
func (m *MessageTestFramework) CreateMessage(messageAlias string, messageOptions ...MessageOption) (message *Message) {
	options := NewMessageTestFrameworkMessageOptions(messageOptions...)

	if transaction := m.buildTransaction(options); transaction != nil {
		m.messagesByAlias[messageAlias] = newTestParentsPayloadMessageIssuer(transaction, m.strongParentIDs(options), m.weakParentIDs(options), options.issuer)
	} else {
		m.messagesByAlias[messageAlias] = newTestParentsDataMessageIssuer(messageAlias, m.strongParentIDs(options), m.weakParentIDs(options), options.issuer)
	}

	RegisterMessageIDAlias(m.messagesByAlias[messageAlias].ID(), messageAlias)

	return m.messagesByAlias[messageAlias]
}

// IncreaseMarkersIndexCallback is the IncreaseMarkersIndexCallback that the MessageTestFramework uses to determine when
// to assign new Markers to messages.
func (m *MessageTestFramework) IncreaseMarkersIndexCallback(markers.SequenceID, markers.Index) bool {
	return false
}

// PreventNewMarkers disables the generation of new Markers for the given Messages.
func (m *MessageTestFramework) PreventNewMarkers(enabled bool) *MessageTestFramework {
	if enabled && m.oldIncreaseIndexCallback == nil {
		m.oldIncreaseIndexCallback = m.tangle.Options.IncreaseMarkersIndexCallback
		m.tangle.Options.IncreaseMarkersIndexCallback = m.IncreaseMarkersIndexCallback
		return m
	}

	if !enabled && m.oldIncreaseIndexCallback != nil {
		m.tangle.Options.IncreaseMarkersIndexCallback = m.oldIncreaseIndexCallback
		m.oldIncreaseIndexCallback = nil
		return m
	}

	return m
}

// IssueMessages stores the given Messages in the Storage and triggers the processing by the Tangle.
func (m *MessageTestFramework) IssueMessages(messageAliases ...string) *MessageTestFramework {
	m.messagesBookedWG.Add(len(messageAliases))
	m.approvalWeightProcessed.Add(len(messageAliases))

	for _, messageAlias := range messageAliases {
		m.tangle.Storage.StoreMessage(m.messagesByAlias[messageAlias])
	}

	return m
}

// WaitMessagesBooked waits for all Messages to be processed by the Booker.
func (m *MessageTestFramework) WaitMessagesBooked() *MessageTestFramework {
	m.messagesBookedWG.Wait()

	return m
}

// WaitApprovalWeightProcessed waits for all Messages to be processed by the ApprovalWeightManager.
func (m *MessageTestFramework) WaitApprovalWeightProcessed() *MessageTestFramework {
	m.approvalWeightProcessed.Wait()

	return m
}

// Message retrieves the Messages that is associated with the given alias.
func (m *MessageTestFramework) Message(alias string) (message *Message) {
	return m.messagesByAlias[alias]
}

// MessageMetadata retrieves the MessageMetadata that is associated with the given alias.
func (m *MessageTestFramework) MessageMetadata(alias string) (messageMetadata *MessageMetadata) {
	m.tangle.Storage.MessageMetadata(m.messagesByAlias[alias].ID()).Consume(func(msgMetadata *MessageMetadata) {
		messageMetadata = msgMetadata
	})

	return
}

// TransactionID returns the TransactionID of the Transaction contained in the Message associated with the given alias.
func (m *MessageTestFramework) TransactionID(messageAlias string) ledgerstate.TransactionID {
	messagePayload := m.messagesByAlias[messageAlias].Payload()
	if messagePayload.Type() != ledgerstate.TransactionType {
		panic(fmt.Sprintf("Message with alias '%s' does not contain a Transaction", messageAlias))
	}

	return messagePayload.(*ledgerstate.Transaction).ID()
}

// BranchID returns the BranchID of the Transaction contained in the Message associated with the given alias.
func (m *MessageTestFramework) BranchID(messageAlias string) ledgerstate.BranchID {
	messagePayload := m.messagesByAlias[messageAlias].Payload()
	if messagePayload.Type() != ledgerstate.TransactionType {
		panic(fmt.Sprintf("Message with alias '%s' does not contain a Transaction", messageAlias))
	}

	return ledgerstate.NewBranchID(messagePayload.(*ledgerstate.Transaction).ID())
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
		addressWallet := createWallets(1)[0]
		m.walletsByAlias[alias] = addressWallet
		m.walletsByAddress[addressWallet.address] = addressWallet

		genesisOutputs[addressWallet.address] = ledgerstate.NewColoredBalances(coloredBalances)
	}

	outputs := []ledgerstate.Output{}

	for address, balance := range genesisOutputs {
		outputs = append(outputs, ledgerstate.NewSigLockedColoredOutput(balance, address))
	}

	genesisEssence := ledgerstate.NewTransactionEssence(
		0,
		time.Now(),
		identity.ID{},
		identity.ID{},
		ledgerstate.NewInputs(ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0))),
		ledgerstate.NewOutputs(outputs...),
	)

	genesisTransaction := ledgerstate.NewTransaction(genesisEssence, ledgerstate.UnlockBlocks{ledgerstate.NewReferenceUnlockBlock(0)})

	snapshot := &ledgerstate.Snapshot{
		Transactions: map[ledgerstate.TransactionID]*ledgerstate.TransactionEssence{
			genesisTransaction.ID(): genesisEssence,
		},
	}

	m.tangle.LedgerState.LoadSnapshot(snapshot)

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
		addressWallet := createWallets(1)[0]
		m.walletsByAlias[alias] = addressWallet
		m.walletsByAddress[addressWallet.address] = addressWallet

		m.outputsByAlias[alias] = ledgerstate.NewSigLockedSingleOutput(balance, m.walletsByAlias[alias].address)

		outputs = append(outputs, m.outputsByAlias[alias])
	}
	for alias, balances := range options.coloredOutputs {
		addressWallet := createWallets(1)[0]
		m.walletsByAlias[alias] = addressWallet
		m.walletsByAddress[addressWallet.address] = addressWallet

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
	for weakParentAlias := range options.weakParents {
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
	issuer         ed25519.PublicKey
}

// NewMessageTestFrameworkMessageOptions is the constructor for the MessageTestFrameworkMessageOptions.
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

// MessageOption is the type that is used for options that can be passed into the CreateMessage method to configure its
// behavior.
type MessageOption func(*MessageTestFrameworkMessageOptions)

// WithInputs returns a MessageOption that is used to provide the Inputs of the Transaction.
func WithInputs(inputAliases ...string) MessageOption {
	return func(options *MessageTestFrameworkMessageOptions) {
		for _, inputAlias := range inputAliases {
			options.inputs[inputAlias] = types.Void
		}
	}
}

// WithOutput returns a MessageOption that is used to define a non-colored Output for the Transaction in the Message.
func WithOutput(alias string, balance uint64) MessageOption {
	return func(options *MessageTestFrameworkMessageOptions) {
		options.outputs[alias] = balance
	}
}

// WithColoredOutput returns a MessageOption that is used to define a colored Output for the Transaction in the Message.
func WithColoredOutput(alias string, balances map[ledgerstate.Color]uint64) MessageOption {
	return func(options *MessageTestFrameworkMessageOptions) {
		options.coloredOutputs[alias] = balances
	}
}

// WithStrongParents returns a MessageOption that is used to define the strong parents of the Message.
func WithStrongParents(messageAliases ...string) MessageOption {
	return func(options *MessageTestFrameworkMessageOptions) {
		for _, messageAlias := range messageAliases {
			options.strongParents[messageAlias] = types.Void
		}
	}
}

// WithWeakParents returns a MessageOption that is used to define the weak parents of the Message.
func WithWeakParents(messageAliases ...string) MessageOption {
	return func(options *MessageTestFrameworkMessageOptions) {
		for _, messageAlias := range messageAliases {
			options.weakParents[messageAlias] = types.Void
		}
	}
}

// WithIssuer returns a MessageOption that is used to define the issuer of the Message.
func WithIssuer(issuer ed25519.PublicKey) MessageOption {
	return func(options *MessageTestFrameworkMessageOptions) {
		options.issuer = issuer
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

func newTestDataMessagePublicKey(payloadString string, publicKey ed25519.PublicKey) *Message {
	return NewMessage([]MessageID{EmptyMessageID}, []MessageID{}, time.Now(), publicKey, nextSequenceNumber(), payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{})
}

func newTestParentsDataMessage(payloadString string, strongParents, weakParents []MessageID) *Message {
	return NewMessage(strongParents, weakParents, time.Now(), ed25519.PublicKey{}, nextSequenceNumber(), payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{})
}

func newTestParentsDataMessageIssuer(payloadString string, strongParents, weakParents []MessageID, issuer ed25519.PublicKey) *Message {
	return NewMessage(strongParents, weakParents, time.Now(), issuer, nextSequenceNumber(), payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{})
}

func newTestParentsDataWithTimestamp(payloadString string, strongParents, weakParents []MessageID, timestamp time.Time) *Message {
	return NewMessage(strongParents, weakParents, timestamp, ed25519.PublicKey{}, nextSequenceNumber(), payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{})
}

func newTestParentsPayloadMessage(p payload.Payload, strongParents, weakParents []MessageID) *Message {
	return NewMessage(strongParents, weakParents, time.Now(), ed25519.PublicKey{}, nextSequenceNumber(), p, 0, ed25519.Signature{})
}

func newTestParentsPayloadMessageIssuer(p payload.Payload, strongParents, weakParents []MessageID, issuer ed25519.PublicKey) *Message {
	return NewMessage(strongParents, weakParents, time.Now(), issuer, nextSequenceNumber(), p, 0, ed25519.Signature{})
}

func newTestParentsPayloadWithTimestamp(p payload.Payload, strongParents, weakParents []MessageID, timestamp time.Time) *Message {
	return NewMessage(strongParents, weakParents, timestamp, ed25519.PublicKey{}, nextSequenceNumber(), p, 0, ed25519.Signature{})
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
	return ledgerstate.NewED25519Signature(w.publicKey(), w.privateKey().Sign(txEssence.Bytes()))
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
	return tangle.Booker.MessageBranchID(messageID)
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
