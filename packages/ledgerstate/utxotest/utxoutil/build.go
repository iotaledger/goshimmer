package utxoutil

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/identity"
	"golang.org/x/xerrors"
	"time"
)

type Builder struct {
	version           ledgerstate.TransactionEssenceVersion
	accessPledgeID    identity.ID
	consensusPledgeID identity.ID
	timestamp         time.Time
	consumables       []*ConsumableOutput
	outputs           []ledgerstate.Output
	consumedUnspent   map[ledgerstate.Color]uint64
}

func NewBuilder(inputs []ledgerstate.Output) *Builder {
	ret := &Builder{
		timestamp:       time.Now(),
		consumables:     make([]*ConsumableOutput, len(inputs)),
		outputs:         make([]ledgerstate.Output, 0),
		consumedUnspent: make(map[ledgerstate.Color]uint64),
	}
	for i, out := range inputs {
		ret.consumables[i] = NewConsumableOutput(out)
	}
	return ret
}

func (b *Builder) WithVersion(v ledgerstate.TransactionEssenceVersion) *Builder {
	b.version = v
	return b
}

func (b *Builder) WithTime(t time.Time) *Builder {
	b.timestamp = t
	return b
}

func (b *Builder) WithAccessPledge(id identity.ID) *Builder {
	b.accessPledgeID = id
	return b
}

func (b *Builder) WithConsensusPledge(id identity.ID) *Builder {
	b.consensusPledgeID = id
	return b
}

func (b *Builder) AddOutput(out ledgerstate.Output) error {
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
		s, _ := b.consumedUnspent[col]
		b.consumedUnspent[col] = s + bal
	}
}

func (b *Builder) consumeAmounts(amounts map[ledgerstate.Color]uint64) bool {
	if !ConsumeAll(amounts, b.consumables...) {
		return false
	}
	b.addToConsumedUnspent(amounts)
	return true
}

func (b *Builder) ensureEnoughUnspendAmounts(amounts map[ledgerstate.Color]uint64) bool {
	missing := make(map[ledgerstate.Color]uint64)
	for col, bal := range amounts {
		if s, _ := b.consumedUnspent[col]; s < bal {
			missing[col] = bal - s
		}
	}
	if len(missing) > 0 {
		if !b.consumeAmounts(missing) {
			return false
		}
	}
	return true
}

func (b *Builder) mustSpendAmounts(amounts map[ledgerstate.Color]uint64) {
	for col, bal := range amounts {
		if s, _ := b.consumedUnspent[col]; s >= bal {
			if s == bal {
				delete(b.consumedUnspent, col)
			} else {
				b.consumedUnspent[col] = s - bal
			}
		} else {
			panic("not enough unspent amounts")
		}
	}
}

// SpendConsumedUnspent spends all consumed unspent and return
func (b *Builder) SpendConsumedUnspent() map[ledgerstate.Color]uint64 {
	ret := b.consumedUnspent
	b.consumedUnspent = make(map[ledgerstate.Color]uint64)
	return ret
}

func (b *Builder) prepareColoredBalancesOutput(amounts map[ledgerstate.Color]uint64, mint ...uint64) (map[ledgerstate.Color]uint64, error) {
	if len(amounts) == 0 {
		return nil, xerrors.New("AddSigLockedColoredOutput: no tokens to transfer")
	}
	// check if it is enough consumed unspent amounts
	if !b.ensureEnoughUnspendAmounts(amounts) {
		return nil, xerrors.New("AddSigLockedColoredOutput: not enough balance")
	}
	b.mustSpendAmounts(amounts)
	amountsCopy := make(map[ledgerstate.Color]uint64)
	for col, bal := range amounts {
		if bal == 0 {
			return nil, xerrors.New("AddSigLockedColoredOutput: zero tokens in input not allowed")
		}
		amountsCopy[col] = bal
	}
	iotas, _ := amountsCopy[ledgerstate.ColorIOTA]
	if len(mint) > 0 && mint[0] > iotas {
		return nil, xerrors.Errorf("can't mint more tokens (%d) than consumed iotas (%d)", iotas, mint[0])
	}
	if len(mint) > 0 && mint[0] > 0 {
		amountsCopy[ledgerstate.ColorMint] = mint[0]
		if iotas > mint[0] {
			amountsCopy[ledgerstate.ColorIOTA] = iotas - mint[0]
		}
	}
	return amountsCopy, nil
}

func (b *Builder) AddSigLockedColoredOutput(targetAddress ledgerstate.Address, amounts map[ledgerstate.Color]uint64, mint ...uint64) error {
	balances, err := b.prepareColoredBalancesOutput(amounts, mint...)
	if err != nil {
		return err
	}
	bals := ledgerstate.NewColoredBalances(balances)
	output := ledgerstate.NewSigLockedColoredOutput(bals, targetAddress)
	if err := b.AddOutput(output); err != nil {
		return err
	}
	return nil
}

// AddSigLockedIOTAOutput adds output with iotas by consuming inputs
// supports minting (coloring) of part of consumed iotas
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
	if err := b.AddOutput(output); err != nil {
		return err
	}
	return nil
}

func (b *Builder) AddExtendedOutputSimple(targetAddress ledgerstate.Address, amounts map[ledgerstate.Color]uint64, mint ...uint64) error {
	balances, err := b.prepareColoredBalancesOutput(amounts, mint...)
	if err != nil {
		return err
	}
	output := ledgerstate.NewExtendedLockedOutput(balances, targetAddress)
	if err := b.AddOutput(output); err != nil {
		return err
	}
	return nil
}

// ConsumeChainInput consumes and returns clone of the input
func (b *Builder) ConsumeChainInput(addressAlias ledgerstate.Address) (ledgerstate.Output, error) {
	out, idx, ok := FindChainInput(addressAlias, b.consumables...)
	if !ok {
		return nil, xerrors.Errorf("can't find chain input for %s", addressAlias)
	}
	b.MustConsumeUntouchedInputByIndex(idx)
	return out.NewChainOutputNext(true), nil
}

func (b *Builder) MustConsumeUntouchedInputByIndex(index int) {
	if index >= len(b.consumables) || b.consumables[index].WasConsumed() {
		panic("MustConsumeUntouchedInputByIndex: invalid consumable")
	}
	b.addToConsumedUnspent(ConsumeRemaining(b.consumables[index]))
}

func (b *Builder) ConsumeReminderBalances(compress bool) []*ConsumableOutput {
	inputConsumables := b.consumables
	if !compress {
		inputConsumables = SelectConsumed(b.consumables...)
	}
	b.addToConsumedUnspent(ConsumeRemaining(inputConsumables...))
	return inputConsumables
}

func (b *Builder) BuildEssence(compress ...bool) (*ledgerstate.TransactionEssence, error) {
	if len(b.consumedUnspent) > 0 {
		return nil, xerrors.New("not all consumed balances were spent")
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
			return nil, xerrors.New("not all inputs were completely consumed")
		}
	}
	// NewOutputs sorts the outputs and changes indices -> impossible to know index of a particular output
	outputs := ledgerstate.NewOutputs(b.outputs...)
	inputs := MakeUTXOInputs(inputConsumables...)
	ret := ledgerstate.NewTransactionEssence(b.version, b.timestamp, b.accessPledgeID, b.consensusPledgeID, inputs, outputs)
	return ret, nil
}

//
//func (b *Builder) BuildWithED25519(keyPair *ed25519.KeyPair) (*ledgerstate.Transaction, error) {
//	addr := ledgerstate.NewED25519Address(keyPair.PublicKey)
//	essence, err := b.BuildEssence(addr)
//	if err != nil {
//		return nil, err
//	}
//	data := essence.Bytes()
//	signature := ledgerstate.NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(data))
//	if !signature.AddressSignatureValid(addr, data) {
//		panic("BuildWithED25519: internal error, signature invalid")
//	}
//	unlockBlocks := unlockBlocksFromSignature(signature, len(essence.Inputs()))
//	return ledgerstate.NewTransaction(essence, unlockBlocks), nil
//}
//
//func unlockBlocksFromSignature(signature ledgerstate.Signature, n int) ledgerstate.UnlockBlocks {
//	ret := make(ledgerstate.UnlockBlocks, n)
//	ret[0] = ledgerstate.NewSignatureUnlockBlock(signature)
//	for i := 1; i < n; i++ {
//		ret[i] = ledgerstate.NewReferenceUnlockBlock(0)
//	}
//	return ret
//}
