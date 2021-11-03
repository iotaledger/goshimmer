package utxoutil

import (
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/client/wallet/packages/sendoptions"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// Builder builder implements a structure and interface to build transactions
// Initial input is a list of outputs which can be used as consumable inputs
// Balances are tracked by "consuming" tokens from inputs and then "spending" them to outputs
// "consuming" tokens from inputs means:
// - tracking "remaining" part of tokens in each consumable output
// - placing consumed token into the "consumedUnspent"
// - By SpendConsumedUnspent all tokens are cleared from "consumedUnspent" and can be used as a token payload to
//   the output
type Builder struct {
	version           ledgerstate.TransactionEssenceVersion
	accessPledgeID    identity.ID
	consensusPledgeID identity.ID
	timestamp         time.Time
	// tracks remaining tokens for each consumable output
	consumables []*ConsumableOutput
	outputs     []ledgerstate.Output
	// buffer of consumed but unspent yet tokens
	consumedUnspent map[ledgerstate.Color]uint64
}

// NewBuilder creates new builder for outputs.
func NewBuilder(inputs ...ledgerstate.Output) *Builder {
	ret := &Builder{
		timestamp:       time.Now(),
		consumables:     make([]*ConsumableOutput, len(inputs)),
		outputs:         make([]ledgerstate.Output, 0),
		consumedUnspent: make(map[ledgerstate.Color]uint64),
	}
	ret.consumables = NewConsumables(inputs...)
	return ret
}

// AddConsumable adds a new consumable output to the builder.
func (b *Builder) AddConsumable(input ledgerstate.Output) {
	b.consumables = append(b.consumables, NewConsumable(input))
}

// Clone creates deep copy of the builder.
func (b *Builder) Clone() *Builder {
	ret := *b
	ret.consumables = make([]*ConsumableOutput, len(b.consumables))
	ret.outputs = make([]ledgerstate.Output, len(b.outputs))
	ret.consumedUnspent = make(map[ledgerstate.Color]uint64)
	for i := range ret.consumables {
		ret.consumables[i] = b.consumables[i].Clone()
	}
	for i := range ret.outputs {
		ret.outputs[i] = b.outputs[i].Clone()
	}
	for col, bal := range b.consumedUnspent {
		ret.consumedUnspent[col] = bal
	}
	return &ret
}

// WithVersion sets Version property.
func (b *Builder) WithVersion(v ledgerstate.TransactionEssenceVersion) *Builder {
	b.version = v
	return b
}

// WithTimestamp sets timestamp.
func (b *Builder) WithTimestamp(t time.Time) *Builder {
	b.timestamp = t
	return b
}

// WithAccessPledge sets the access mana pledge.
func (b *Builder) WithAccessPledge(id identity.ID) *Builder {
	b.accessPledgeID = id
	return b
}

// WithConsensusPledge sets the consensus mana pledge.
func (b *Builder) WithConsensusPledge(id identity.ID) *Builder {
	b.consensusPledgeID = id
	return b
}

// AddOutputAndSpendUnspent spends the consumed-unspent tokens and adds output.
func (b *Builder) AddOutputAndSpendUnspent(out ledgerstate.Output) error {
	b.SpendConsumedUnspent()
	return b.addOutput(out)
}

func (b *Builder) addOutput(out ledgerstate.Output) error {
	for _, o := range b.outputs {
		if out.Compare(o) == 0 {
			return xerrors.New("duplicate outputs not allowed")
		}
	}
	b.outputs = append(b.outputs, out)
	return nil
}

func (b *Builder) addToConsumedUnspent(bals map[ledgerstate.Color]uint64) {
	for col, bal := range bals {
		s := b.consumedUnspent[col]
		b.consumedUnspent[col] = s + bal
	}
}

// ConsumeAmounts consumes specified amounts and adds it to consumed-unspent pool.
func (b *Builder) ConsumeAmounts(amounts map[ledgerstate.Color]uint64) bool {
	if !ConsumeMany(amounts, b.consumables...) {
		return false
	}
	b.addToConsumedUnspent(amounts)
	return true
}

func (b *Builder) ensureEnoughUnspendAmounts(amounts map[ledgerstate.Color]uint64) bool {
	missing := make(map[ledgerstate.Color]uint64)
	for col, bal := range amounts {
		if s := b.consumedUnspent[col]; s < bal {
			missing[col] = bal - s
		}
	}
	if len(missing) > 0 {
		if !b.ConsumeAmounts(missing) {
			return false
		}
	}
	return true
}

func (b *Builder) mustSpendAmounts(amounts map[ledgerstate.Color]uint64) {
	for col, bal := range amounts {
		if s := b.consumedUnspent[col]; s >= bal {
			if s == bal {
				delete(b.consumedUnspent, col)
			} else {
				b.consumedUnspent[col] = s - bal
			}
		} else {
			panic("mustSpendAmounts: not enough unspent amounts")
		}
	}
}

// SpendConsumedUnspent spends all consumed-unspent pool (empties it) and returns what was spent.
func (b *Builder) SpendConsumedUnspent() map[ledgerstate.Color]uint64 {
	ret := b.consumedUnspent
	b.consumedUnspent = make(map[ledgerstate.Color]uint64)
	return ret
}

// Spend spends from consumed-unspend. Return an error if not enough funds.
func (b *Builder) Spend(spend map[ledgerstate.Color]uint64) error {
	// check if enough
	for color, needed := range spend {
		available := b.consumedUnspent[color]
		if available < needed {
			return xerrors.New("Spend: not enough consumed-unspend funds")
		}
	}
	for col, bal := range spend {
		b.consumedUnspent[col] -= bal
	}
	return nil
}

// AddExtendedOutputSpend adds extended output using unspent amounts and spends it. Fails of not enough.
// Do not consume inputs.
func (b *Builder) AddExtendedOutputSpend(
	targetAddress ledgerstate.Address,
	data []byte,
	amounts map[ledgerstate.Color]uint64,
	options *sendoptions.SendFundsOptions,
	mint ...uint64,
) error {
	for col, needed := range amounts {
		available := b.consumedUnspent[col]
		if available < needed {
			return xerrors.New("AddExtendedOutputSpend: not enough consumed-unspent funds")
		}
	}
	if err := b.Spend(amounts); err != nil {
		return err
	}
	output := ledgerstate.NewExtendedLockedOutput(amounts, targetAddress)
	if options != nil {
		if options.FallbackAddress != nil && !options.FallbackDeadline.IsZero() {
			output = output.WithFallbackOptions(options.FallbackAddress, options.FallbackDeadline)
		}
		if !options.LockUntil.IsZero() {
			output = output.WithTimeLock(options.LockUntil)
		}
	}
	if err := output.SetPayload(data); err != nil {
		return err
	}
	if err := b.addOutput(output); err != nil {
		return err
	}
	return nil
}

// prepareColoredBalancesOutput:
// - ensures enough tokens in unspentAmounts
// - spends them
// - handles minting of new colors (if any) and returns final map of tokens with valid minting.
func (b *Builder) prepareColoredBalancesOutput(amounts map[ledgerstate.Color]uint64, mint ...uint64) (map[ledgerstate.Color]uint64, error) {
	if len(amounts) == 0 {
		return nil, xerrors.New("prepareColoredBalancesOutput: no tokens to transfer")
	}
	minting := len(mint) > 0 && mint[0] > 0
	iotas := amounts[ledgerstate.ColorIOTA]
	if minting && iotas < mint[0] {
		return nil, xerrors.Errorf("prepareColoredBalancesOutput: not enough iotas (%d) to mint %d new colored tokens", iotas, mint[0])
	}
	// check if it is enough consumed unspent amounts
	if !b.ensureEnoughUnspendAmounts(amounts) {
		return nil, xerrors.New("prepareColoredBalancesOutput: not enough balance")
	}
	b.mustSpendAmounts(amounts)
	amountsCopy := make(map[ledgerstate.Color]uint64)
	for col, bal := range amounts {
		if bal == 0 {
			return nil, xerrors.New("prepareColoredBalancesOutput: zero tokens in input not allowed")
		}
		amountsCopy[col] = bal
	}
	if minting {
		amountsCopy[ledgerstate.ColorMint] = mint[0]
		amountsCopy[ledgerstate.ColorIOTA] = iotas - mint[0]
		if amountsCopy[ledgerstate.ColorIOTA] == 0 {
			delete(amountsCopy, ledgerstate.ColorIOTA)
		}
	}
	return amountsCopy, nil
}

// AddSigLockedColoredOutput creates output, consumes inputs if needed.
func (b *Builder) AddSigLockedColoredOutput(targetAddress ledgerstate.Address, amounts map[ledgerstate.Color]uint64, mint ...uint64) error {
	balances, err := b.prepareColoredBalancesOutput(amounts, mint...)
	if err != nil {
		return err
	}
	bals := ledgerstate.NewColoredBalances(balances)
	output := ledgerstate.NewSigLockedColoredOutput(bals, targetAddress)
	if err := b.addOutput(output); err != nil {
		return err
	}
	return nil
}

// AddSigLockedIOTAOutput adds output with iotas by consuming inputs
// supports minting (coloring) of part of consumed iotas.
func (b *Builder) AddSigLockedIOTAOutput(targetAddress ledgerstate.Address, amount uint64, mint ...uint64) error {
	balances, err := b.prepareColoredBalancesOutput(map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: amount}, mint...)
	if err != nil {
		return err
	}
	var output ledgerstate.Output
	if _, ok := balances[ledgerstate.ColorMint]; ok {
		output = ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(balances), targetAddress)
	} else {
		output = ledgerstate.NewSigLockedSingleOutput(amount, targetAddress)
	}
	if err := b.addOutput(output); err != nil {
		return err
	}
	return nil
}

// AddExtendedOutputConsume add new output. Ensures enough unspent funds by consuming if necessary.
func (b *Builder) AddExtendedOutputConsume(targetAddress ledgerstate.Address, data []byte, amounts map[ledgerstate.Color]uint64, mint ...uint64) error {
	balances, err := b.prepareColoredBalancesOutput(amounts, mint...)
	if err != nil {
		return err
	}
	output := ledgerstate.NewExtendedLockedOutput(balances, targetAddress)
	if err := output.SetPayload(data); err != nil {
		return err
	}
	if err := b.addOutput(output); err != nil {
		return err
	}
	return nil
}

// AddRemainderOutputIfNeeded consumes already touched inputs and spends consumed-unspend.
// Creates remainder output if needed.
func (b *Builder) AddRemainderOutputIfNeeded(remainderAddr ledgerstate.Address, data []byte, compress ...bool) error {
	compr := false
	if len(compress) > 0 {
		compr = compress[0]
	}
	b.ConsumeRemainingBalances(compr)
	unspent := b.ConsumedUnspent()
	if len(unspent) == 0 {
		// no need for remainder output
		return nil
	}
	return b.AddExtendedOutputConsume(remainderAddr, data, unspent)
}

// AddMintingOutputConsume mints new tokens. Consumes additional iotas if needed.
func (b *Builder) AddMintingOutputConsume(targetAddress ledgerstate.Address, amount uint64, payload ...[]byte) error {
	amounts := map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: amount}
	if !b.ensureEnoughUnspendAmounts(amounts) {
		return xerrors.New("AddMintingOutputConsumer: not enough balance")
	}
	b.mustSpendAmounts(amounts)
	// iotas replaced with minting color
	amounts = map[ledgerstate.Color]uint64{ledgerstate.ColorMint: amount}
	output := ledgerstate.NewExtendedLockedOutput(amounts, targetAddress)
	if len(payload) > 0 {
		if err := output.SetPayload(payload[0]); err != nil {
			return err
		}
	}
	if err := b.addOutput(output); err != nil {
		return err
	}
	return nil
}

// AddNewAliasMint creates new self governed chain.
// The identity of the chain is not known until the full transaction is produced.
func (b *Builder) AddNewAliasMint(balances map[ledgerstate.Color]uint64, stateAddress ledgerstate.Address, stateData []byte) error {
	output, err := ledgerstate.NewAliasOutputMint(balances, stateAddress)
	if err != nil {
		return err
	}
	if err := output.SetStateData(stateData); err != nil {
		return err
	}
	if !b.ensureEnoughUnspendAmounts(balances) {
		return xerrors.New("AddNewAliasMint: not enough tokens")
	}
	b.SpendConsumedUnspent()
	if err := b.addOutput(output); err != nil {
		return err
	}
	return nil
}

// ConsumedUnspent return consumed-unspend pool.
func (b *Builder) ConsumedUnspent() map[ledgerstate.Color]uint64 {
	ret := make(map[ledgerstate.Color]uint64)
	for col, bal := range b.consumedUnspent {
		if bal != 0 {
			ret[col] = bal
		}
	}
	return ret
}

// ConsumeAliasInput consumes chain input by alias.
func (b *Builder) ConsumeAliasInput(addressAlias ledgerstate.Address) error {
	out, _, ok := FindAliasConsumableInput(addressAlias, b.consumables...)
	if !ok {
		return xerrors.Errorf("ConsumeAliasInput: can't find chain input for %s", addressAlias)
	}
	if err := b.ConsumeInputByOutputID(out.ID()); err != nil {
		return err
	}
	return nil
}

// AliasNextChainedOutput creates chained output without consuming it.
func (b *Builder) AliasNextChainedOutput(addressAlias ledgerstate.Address) (*ledgerstate.AliasOutput, error) {
	out, _, ok := FindAliasConsumableInput(addressAlias, b.consumables...)
	if !ok {
		return nil, xerrors.Errorf("can't find chain input for %s", addressAlias)
	}
	return out.NewAliasOutputNext(false), nil
}

// AddAliasOutputAsRemainder forms a remainder by creating new alias output.
func (b *Builder) AddAliasOutputAsRemainder(addressAlias ledgerstate.Address, stateData []byte, compress ...bool) error {
	out, _, ok := FindAliasConsumableInput(addressAlias, b.consumables...)
	if !ok {
		return xerrors.Errorf("can't find chain input for %s", addressAlias)
	}
	if err := b.ConsumeInputByOutputID(out.ID()); err != nil {
		return err
	}
	compr := false
	if len(compress) > 0 {
		compr = compress[0]
	}
	b.ConsumeRemainingBalances(compr)
	chained := out.NewAliasOutputNext()
	if err := chained.SetBalances(b.ConsumedUnspent()); err != nil {
		return err
	}
	if err := chained.SetStateData(stateData); err != nil {
		return err
	}
	if err := b.AddOutputAndSpendUnspent(chained); err != nil {
		return err
	}
	return nil
}

// ConsumeInputByOutputID consumes input by outputID.
func (b *Builder) ConsumeInputByOutputID(id ledgerstate.OutputID) error {
	for _, consumable := range b.consumables {
		if consumable.output.ID() == id {
			b.addToConsumedUnspent(ConsumeRemaining(consumable))
			return nil
		}
	}
	return xerrors.Errorf("ConsumeInputByOutputID: output not found")
}

// ConsumeRemainingBalances consumes touched balances.
func (b *Builder) ConsumeRemainingBalances(compress bool) []*ConsumableOutput {
	inputConsumables := b.consumables
	if !compress {
		inputConsumables = SelectConsumed(b.consumables...)
	}
	b.addToConsumedUnspent(ConsumeRemaining(inputConsumables...))
	return inputConsumables
}

// BuildEssence builds essence of the transaction.
// Compress option:
// - true means take all consumable inputs
// - false means take only touched (consumed) outputs. This is the default.
func (b *Builder) BuildEssence(compress ...bool) (*ledgerstate.TransactionEssence, []ledgerstate.Output, error) {
	if len(b.consumedUnspent) > 0 {
		return nil, nil, xerrors.New("BuildEssence: not all consumed balances were spent")
	}
	compr := false
	if len(compress) > 0 {
		compr = compress[0]
	}
	inputConsumables := b.consumables
	if !compr {
		inputConsumables = SelectConsumed(b.consumables...)
	}
	for _, consumable := range inputConsumables {
		if !consumable.NothingRemains() {
			return nil, nil, xerrors.New("BuildEssence: not all inputs were completely were consumed")
		}
	}
	// NewOutputs sorts the outputs and changes indices -> impossible to know indexUnlocked of a particular output
	outputs := ledgerstate.NewOutputs(b.outputs...)
	inputs, consumedOutputs := MakeUTXOInputs(inputConsumables...)
	ret := ledgerstate.NewTransactionEssence(b.version, b.timestamp, b.accessPledgeID, b.consensusPledgeID, inputs, outputs)
	return ret, consumedOutputs, nil
}

// BuildWithED25519 build complete transaction and signs/unlocks with provided keys.
func (b *Builder) BuildWithED25519(keyPairs ...*ed25519.KeyPair) (*ledgerstate.Transaction, error) {
	essence, consumedOutputs, err := b.BuildEssence()
	if err != nil {
		return nil, err
	}
	unlockBlocks, err2 := UnlockInputsWithED25519KeyPairs(consumedOutputs, essence, keyPairs...)
	if err2 != nil {
		return nil, err2
	}
	return ledgerstate.NewTransaction(essence, unlockBlocks), nil
}
