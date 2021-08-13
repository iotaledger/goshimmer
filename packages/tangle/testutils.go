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

	"github.com/iotaledger/goshimmer/packages/consensus"
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

// region MessageTestFramework /////////////////////////////////////////////////////////////////////////////////////////

// MessageTestFramework implements a framework for conveniently issuing messages in a tangle as part of unit tests in a
// simplified way.
type MessageTestFramework struct {
	tangle                   *Tangle
	branchIDs                map[string]ledgerstate.BranchID
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
		branchIDs:        make(map[string]ledgerstate.BranchID),
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

// RegisterBranchID registers a BranchID from the given Messages' transactions with the MessageTestFramework and
// also an alias when printing the BranchID.
func (m *MessageTestFramework) RegisterBranchID(alias string, messageAliases ...string) {
	if len(messageAliases) == 1 {
		branchID := m.BranchIDFromMessage(messageAliases[0])
		m.branchIDs[alias] = branchID
		ledgerstate.RegisterBranchIDAlias(branchID, alias)
		return
	}

	aggregation := ledgerstate.NewBranchIDs()
	for _, messageAlias := range messageAliases {
		branch := m.BranchIDFromMessage(messageAlias)
		aggregation.Add(branch)
	}
	cachedAggregatedBranch, _, err := m.tangle.LedgerState.BranchDAG.AggregateBranches(aggregation)
	if err != nil {
		panic("my mom beats me with an aggregated branch")
	}
	defer cachedAggregatedBranch.Release()
	branchID := cachedAggregatedBranch.ID()
	m.branchIDs[alias] = branchID
	ledgerstate.RegisterBranchIDAlias(branchID, alias)
}

// BranchID returns the BranchID registered with the given alias.
func (m *MessageTestFramework) BranchID(alias string) ledgerstate.BranchID {
	branchID, ok := m.branchIDs[alias]
	if !ok {
		panic("no branch registered with such alias " + alias)
	}
	return branchID
}

// CreateMessage creates a Message with the given alias and MessageTestFrameworkMessageOptions.
func (m *MessageTestFramework) CreateMessage(messageAlias string, messageOptions ...MessageOption) (message *Message) {
	options := NewMessageTestFrameworkMessageOptions(messageOptions...)

	if options.reattachmentMessageAlias != "" {
		reattachmentPayload := m.Message(options.reattachmentMessageAlias).Payload()
		if options.issuingTime.IsZero() {
			m.messagesByAlias[messageAlias] = newTestParentsPayloadMessageIssuer(reattachmentPayload, m.strongParentIDs(options), m.weakParentIDs(options), nil, m.likeParentIDs(options), options.issuer)
		} else {
			m.messagesByAlias[messageAlias] = newTestParentsPayloadMessageTimestampIssuer(reattachmentPayload, m.strongParentIDs(options), m.weakParentIDs(options), nil, m.likeParentIDs(options), options.issuer, options.issuingTime)
		}
	} else {
		transaction := m.buildTransaction(options)

		if transaction != nil && options.issuingTime.IsZero() {
			m.messagesByAlias[messageAlias] = newTestParentsPayloadMessageIssuer(transaction, m.strongParentIDs(options), m.weakParentIDs(options), nil, m.likeParentIDs(options), options.issuer)
		} else if transaction != nil && !options.issuingTime.IsZero() {
			m.messagesByAlias[messageAlias] = newTestParentsPayloadMessageTimestampIssuer(transaction, m.strongParentIDs(options), m.weakParentIDs(options), nil, m.likeParentIDs(options), options.issuer, options.issuingTime)
		} else if options.issuingTime.IsZero() {
			m.messagesByAlias[messageAlias] = newTestParentsDataMessageIssuer(messageAlias, m.strongParentIDs(options), m.weakParentIDs(options), nil, m.likeParentIDs(options), options.issuer)
		} else {
			m.messagesByAlias[messageAlias] = newTestParentsDataMessageTimestampIssuer(messageAlias, m.strongParentIDs(options), m.weakParentIDs(options), nil, m.likeParentIDs(options), options.issuer, options.issuingTime)
		}
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

// BranchIDFromMessage returns the BranchID of the Transaction contained in the Message associated with the given alias.
func (m *MessageTestFramework) BranchIDFromMessage(messageAlias string) ledgerstate.BranchID {
	messagePayload := m.messagesByAlias[messageAlias].Payload()
	if messagePayload.Type() != ledgerstate.TransactionType {
		panic(fmt.Sprintf("Message with alias '%s' does not contain a Transaction", messageAlias))
	}

	return ledgerstate.NewBranchID(messagePayload.(*ledgerstate.Transaction).ID())
}

// createGenesisOutputs initializes the Outputs that are used by the MessageTestFramework as the genesis.
func (m *MessageTestFramework) createGenesisOutputs() {
	if len(m.options.genesisOutputs) == 0 {
		return
	}

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

	var outputs []ledgerstate.Output
	var unspentOutputs []bool

	for address, balance := range genesisOutputs {
		outputs = append(outputs, ledgerstate.NewSigLockedColoredOutput(balance, address))
		unspentOutputs = append(unspentOutputs, true)
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
		Transactions: map[ledgerstate.TransactionID]ledgerstate.Record{
			genesisTransaction.ID(): {
				Essence:        genesisEssence,
				UnlockBlocks:   ledgerstate.UnlockBlocks{ledgerstate.NewReferenceUnlockBlock(0)},
				UnspentOutputs: unspentOutputs,
			},
		},
	}

	if err := m.tangle.LedgerState.LoadSnapshot(snapshot); err != nil {
		panic(err)
	}

	for alias := range m.options.genesisOutputs {
		m.tangle.LedgerState.UTXODAG.CachedAddressOutputMapping(m.walletsByAlias[alias].address).Consume(func(addressOutputMapping *ledgerstate.AddressOutputMapping) {
			m.tangle.LedgerState.UTXODAG.CachedOutput(addressOutputMapping.OutputID()).Consume(func(output ledgerstate.Output) {
				m.outputsByAlias[alias] = output
				m.outputsByID[addressOutputMapping.OutputID()] = output
				m.inputsByAlias[alias] = ledgerstate.NewUTXOInput(addressOutputMapping.OutputID())
			})
		})
	}

	for alias := range m.options.coloredGenesisOutputs {
		m.tangle.LedgerState.UTXODAG.CachedAddressOutputMapping(m.walletsByAlias[alias].address).Consume(func(addressOutputMapping *ledgerstate.AddressOutputMapping) {
			m.tangle.LedgerState.UTXODAG.CachedOutput(addressOutputMapping.OutputID()).Consume(func(output ledgerstate.Output) {
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

// likeParentIDs returns the MessageIDs that were defined to be the like parents of the
// MessageTestFrameworkMessageOptions.
func (m *MessageTestFramework) likeParentIDs(options *MessageTestFrameworkMessageOptions) (likeParentIDs MessageIDs) {
	likeParentIDs = make(MessageIDs, 0)
	for likeParentAlias := range options.likeParents {
		if likeParentAlias == "Genesis" {
			likeParentIDs = append(likeParentIDs, EmptyMessageID)

			continue
		}

		likeParentIDs = append(likeParentIDs, m.messagesByAlias[likeParentAlias].ID())
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
	inputs                   map[string]types.Empty
	outputs                  map[string]uint64
	coloredOutputs           map[string]map[ledgerstate.Color]uint64
	strongParents            map[string]types.Empty
	weakParents              map[string]types.Empty
	likeParents              map[string]types.Empty
	issuer                   ed25519.PublicKey
	issuingTime              time.Time
	reattachmentMessageAlias string
}

// NewMessageTestFrameworkMessageOptions is the constructor for the MessageTestFrameworkMessageOptions.
func NewMessageTestFrameworkMessageOptions(options ...MessageOption) (messageOptions *MessageTestFrameworkMessageOptions) {
	messageOptions = &MessageTestFrameworkMessageOptions{
		inputs:        make(map[string]types.Empty),
		outputs:       make(map[string]uint64),
		strongParents: make(map[string]types.Empty),
		weakParents:   make(map[string]types.Empty),
		likeParents:   make(map[string]types.Empty),
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

// WithLikeParents returns a MessageOption that is used to define the like parents of the Message.
func WithLikeParents(messageAliases ...string) MessageOption {
	return func(options *MessageTestFrameworkMessageOptions) {
		for _, messageAlias := range messageAliases {
			options.likeParents[messageAlias] = types.Void
		}
	}
}

// WithIssuer returns a MessageOption that is used to define the issuer of the Message.
func WithIssuer(issuer ed25519.PublicKey) MessageOption {
	return func(options *MessageTestFrameworkMessageOptions) {
		options.issuer = issuer
	}
}

// WithIssuingTime returns a MessageOption that is used to set issuing time of the Message.
func WithIssuingTime(issuingTime time.Time) MessageOption {
	return func(options *MessageTestFrameworkMessageOptions) {
		options.issuingTime = issuingTime
	}
}

// WithReattachment returns a MessageOption that is used to select payload of which Message should be reattached.
func WithReattachment(messageAlias string) MessageOption {
	return func(options *MessageTestFrameworkMessageOptions) {
		options.reattachmentMessageAlias = messageAlias
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Utility functions ////////////////////////////////////////////////////////////////////////////////////////////

var sequenceNumber uint64

func nextSequenceNumber() uint64 {
	return atomic.AddUint64(&sequenceNumber, 1) - 1
}

func newTestNonceMessage(nonce uint64) *Message {
	return NewMessage([]MessageID{EmptyMessageID}, []MessageID{}, nil, nil, time.Time{}, ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte("test")), nonce, ed25519.Signature{})
}

func newTestDataMessage(payloadString string) *Message {
	return NewMessage([]MessageID{EmptyMessageID}, []MessageID{}, nil, nil, time.Now(), ed25519.PublicKey{}, nextSequenceNumber(), payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{})
}

func newTestDataMessagePublicKey(payloadString string, publicKey ed25519.PublicKey) *Message {
	return NewMessage([]MessageID{EmptyMessageID}, []MessageID{}, nil, nil, time.Now(), publicKey, nextSequenceNumber(), payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{})
}

func newTestParentsDataMessage(payloadString string, strongParents, weakParents, dislikeParents, likeParents []MessageID) *Message {
	return NewMessage(strongParents, weakParents, dislikeParents, likeParents, time.Now(), ed25519.PublicKey{}, nextSequenceNumber(), payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{})
}

func newTestParentsDataMessageIssuer(payloadString string, strongParents, weakParents, dislikeParents, likeParents []MessageID, issuer ed25519.PublicKey) *Message {
	return NewMessage(strongParents, weakParents, dislikeParents, likeParents, time.Now(), issuer, nextSequenceNumber(), payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{})
}

func newTestParentsDataWithTimestamp(payloadString string, strongParents, weakParents, dislikeParents, likeParents []MessageID, timestamp time.Time) *Message {
	return NewMessage(strongParents, weakParents, dislikeParents, likeParents, timestamp, ed25519.PublicKey{}, nextSequenceNumber(), payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{})
}

func newTestParentsDataMessageTimestampIssuer(payloadString string, strongParents, weakParents, dislikeParents, likeParents []MessageID, issuer ed25519.PublicKey, timestamp time.Time) *Message {
	return NewMessage(strongParents, weakParents, dislikeParents, likeParents, timestamp, issuer, nextSequenceNumber(), payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{})
}

func newTestParentsPayloadMessage(p payload.Payload, strongParents, weakParents, dislikeParents, likeParents []MessageID) *Message {
	return NewMessage(strongParents, weakParents, dislikeParents, likeParents, time.Now(), ed25519.PublicKey{}, nextSequenceNumber(), p, 0, ed25519.Signature{})
}

func newTestParentsPayloadMessageIssuer(p payload.Payload, strongParents, weakParents, dislikeParents, likeParents []MessageID, issuer ed25519.PublicKey) *Message {
	return NewMessage(strongParents, weakParents, dislikeParents, likeParents, time.Now(), issuer, nextSequenceNumber(), p, 0, ed25519.Signature{})
}

func newTestParentsPayloadMessageTimestampIssuer(p payload.Payload, strongParents, weakParents, dislikeParents, likeParents []MessageID, issuer ed25519.PublicKey, timestamp time.Time) *Message {
	return NewMessage(strongParents, weakParents, dislikeParents, likeParents, timestamp, issuer, nextSequenceNumber(), p, 0, ed25519.Signature{})
}

func newTestParentsPayloadWithTimestamp(p payload.Payload, strongParents, weakParents, dislikeParents, likeParents []MessageID, timestamp time.Time) *Message {
	return NewMessage(strongParents, weakParents, dislikeParents, likeParents, timestamp, ed25519.PublicKey{}, nextSequenceNumber(), p, 0, ed25519.Signature{})
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

var (
	aMana               = 1.0
	totalAMana          = 1000.0
	testMaxBuffer       = 1 * 1024 * 1024
	testRate            = time.Second / 5000
	noAManaNode         = identity.GenerateIdentity()
	testSchedulerParams = SchedulerParams{
		MaxBufferSize:               testMaxBuffer,
		Rate:                        testRate,
		AccessManaRetrieveFunc:      accessManaRetriever,
		TotalAccessManaRetrieveFunc: totalAccessManaRetriever,
	}
)

func accessManaRetriever(id identity.ID) float64 {
	if id == noAManaNode.ID() {
		return 0
	}
	return aMana
}

func totalAccessManaRetriever() float64 {
	return totalAMana
}

// NewTestTangle returns a Tangle instance with a testing schedulerConfig
func NewTestTangle(options ...Option) *Tangle {
	cacheTimeProvider := database.NewCacheTimeProvider(0)

	options = append(options, SchedulerConfig(testSchedulerParams), CacheTimeProvider(cacheTimeProvider))

	t := New(options...)
	t.ConfirmationOracle = &MockConfirmationOracle{}

	return t
}

// MockConfirmationOracle is a mock of a ConfirmationOracle.
type MockConfirmationOracle struct{}

// IsMarkerConfirmed mocks its interface function.
func (m *MockConfirmationOracle) IsMarkerConfirmed(*markers.Marker) bool {
	// We do not use the optimization in the AW manager via map for tests. Thus, in the test it always needs to start checking from the
	// beginning of the sequence for all markers.
	return false
}

// IsMessageConfirmed mocks its interface function.
func (m *MockConfirmationOracle) IsMessageConfirmed(msgId MessageID) bool {
	return false
}

// IsBranchConfirmed mocks its interface function.
func (m *MockConfirmationOracle) IsBranchConfirmed(branchId ledgerstate.BranchID) bool {
	return false
}

// Events mocks its interface function.
func (m *MockConfirmationOracle) Events() *ConfirmationEvents {
	return &ConfirmationEvents{
		MessageConfirmed:     events.NewEvent(nil),
		TransactionConfirmed: events.NewEvent(nil),
		BranchConfirmed:      events.NewEvent(nil),
	}
}

// SimpleMockOnTangleVoting is mock of OTV mechanism.
type SimpleMockOnTangleVoting struct {
	disliked     ledgerstate.BranchIDs
	likedInstead map[ledgerstate.BranchID][]consensus.OpinionTuple
}

// Opinion returns liked and disliked branches as predefined.
func (o *SimpleMockOnTangleVoting) Opinion(branchIDs ledgerstate.BranchIDs) (liked, disliked ledgerstate.BranchIDs, err error) {
	liked = ledgerstate.NewBranchIDs()
	disliked = ledgerstate.NewBranchIDs()
	for branchID := range branchIDs {
		if o.disliked.Contains(branchID) {
			disliked.Add(branchID)
		} else {
			liked.Add(branchID)
		}
	}
	return
}

// LikedInstead returns branches that are liked instead of a disliked branch as predefined.
func (o *SimpleMockOnTangleVoting) LikedInstead(branchID ledgerstate.BranchID) (opinionTuple []consensus.OpinionTuple, err error) {
	opinionTuple = o.likedInstead[branchID]
	return
}

func emptyLikeReferences(parents MessageIDs, issuingTime time.Time, tangle *Tangle) (MessageIDs, error) {
	return []MessageID{}, nil
}
