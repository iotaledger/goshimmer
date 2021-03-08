package utxoutil

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/crypto/ed25519"
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
	senderAddress     ledgerstate.Address
	compress          bool // default is do not compress all outputs to few, only take minimum for outputs
	outputs           []ledgerstate.Output
}

func NewBuilder(inputs []ledgerstate.Output) *Builder {
	senderAddr, err := takeSenderAddress(inputs)
	if err != nil {
		return nil
	}
	ret := &Builder{
		timestamp:     time.Now(),
		consumables:   make([]*ConsumableOutput, len(inputs)),
		senderAddress: senderAddr,
		outputs:       make([]ledgerstate.Output, 0),
	}
	for i, out := range inputs {
		ret.consumables[i] = NewConsumableOutput(out)
	}
	return ret
}

func takeSenderAddress(outputs []ledgerstate.Output) (ledgerstate.Address, error) {
	var ret ledgerstate.Address
	for _, out := range outputs {
		if ret != nil && ret.Array() != out.Address().Array() {
			return nil, xerrors.New("txbuilder.takeSenderAddress: all outputs must be from the same address")
		}
		ret = out.Address()
	}
	return ret, nil
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

func (b *Builder) WithOutputCompression(compress bool) *Builder {
	b.compress = compress
	return b
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

// AddIOTAOutput adds output with iotas by consuming inputs
// supports minting (coloring) of part of consumed iotas
func (b *Builder) AddIOTAOutput(targetAddress ledgerstate.Address, amount uint64, mint ...uint64) (uint16, error) {
	if amount == 0 {
		return 0, xerrors.New("can't add output with 0 iotas")
	}
	if len(mint) > 0 && mint[0] > amount {
		return 0, xerrors.Errorf("can't mint more tokens (%d) than consumed iotas (%d)", amount, mint[0])
	}
	if !ConsumeIOTA(amount, b.consumables...) {
		return 0, xerrors.New("AddIOTAOutput: not enough balance")
	}
	var output ledgerstate.Output
	if len(mint) > 0 && mint[0] > 0 {
		bmap := map[ledgerstate.Color]uint64{
			ledgerstate.ColorMint: mint[0],
		}
		if amount > mint[0] {
			bmap[ledgerstate.ColorIOTA] = amount - mint[0]
		}
		output = ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(bmap), targetAddress)
	} else {
		output = ledgerstate.NewSigLockedSingleOutput(amount, targetAddress)
	}
	if err := b.addOutput(output); err != nil {
		return 0, err
	}
	return uint16(len(b.outputs) - 1), nil
}

// AddSimpleColoredOutput adds output with colored tokens by consuming inputs
// Supports minting (coloring) of part of consumed iotas. Re-coloring of other colors do not support
func (b *Builder) AddSimpleColoredOutput(targetAddress ledgerstate.Address, amounts map[ledgerstate.Color]uint64, mint ...uint64) (uint16, error) {
	if len(amounts) == 0 {
		return 0, xerrors.New("AddSimpleColoredOutput: no tokens to transfer")
	}
	amountsCopy := make(map[ledgerstate.Color]uint64)
	for col, bal := range amounts {
		if bal == 0 {
			return 0, xerrors.New("AddSimpleColoredOutput: zero tokens in input not allowed")
		}
		amountsCopy[col] = bal
	}
	iotas, _ := amountsCopy[ledgerstate.ColorIOTA]
	if len(mint) > 0 && mint[0] > iotas {
		return 0, xerrors.Errorf("can't mint more tokens (%d) than consumed iotas (%d)", iotas, mint[0])
	}
	if !ConsumeAll(amountsCopy, b.consumables...) {
		return 0, xerrors.New("AddSimpleColoredOutput: not enough balance")
	}
	if len(mint) > 0 && mint[0] > 0 {
		amountsCopy[ledgerstate.ColorMint] = mint[0]
		if iotas > mint[0] {
			amountsCopy[ledgerstate.ColorIOTA] = iotas - mint[0]
		}
	}
	bals := ledgerstate.NewColoredBalances(amountsCopy)
	output := ledgerstate.NewSigLockedColoredOutput(bals, targetAddress)
	if err := b.addOutput(output); err != nil {
		return 0, err
	}
	return uint16(len(b.outputs) - 1), nil
}

// AddTransferFromUnconsumedInputs this is used by VM.
// Listed untouched inputs are all consumed and sent to the same output
func (b *Builder) AddTransferFromUnconsumedInputs(targetAddress ledgerstate.Address, inputIndices ...uint16) error {
	inputs := make([]*ConsumableOutput, len(inputIndices))
	for i, idx := range inputIndices {
		if int(idx) >= len(b.consumables) || len(b.consumables[idx].consumed) > 0 {
			return xerrors.New("AddTransferFromUnconsumedInput: wrong input index or input already consumed")
		}
		inputs[i] = b.consumables[idx]
	}
	transferredTotals := ConsumeRemaining(inputs...)
	var output ledgerstate.Output
	if len(transferredTotals) == 1 {
		if iotas, ok := transferredTotals[ledgerstate.ColorIOTA]; ok {
			output = ledgerstate.NewSigLockedSingleOutput(iotas, targetAddress)
		}
	}
	if output == nil {
		output = ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(transferredTotals), targetAddress)
	}
	if err := b.addOutput(output); err != nil {
		return err
	}
	return nil
}

func (b *Builder) addReminderOutput() ([]*ConsumableOutput, error) {
	inputConsumables := b.consumables
	if !b.compress {
		inputConsumables = SelectConsumed(b.consumables...)
	}
	reminderBalances := ConsumeRemaining(inputConsumables...)
	if len(reminderBalances) != 0 {
		var output ledgerstate.Output
		if numIotas, ok := reminderBalances[ledgerstate.ColorIOTA]; ok && len(reminderBalances) == 1 {
			output = ledgerstate.NewSigLockedSingleOutput(numIotas, b.senderAddress)
		} else {
			bals := ledgerstate.NewColoredBalances(reminderBalances)
			output = ledgerstate.NewSigLockedColoredOutput(bals, b.senderAddress)
		}
		if err := b.addOutput(output); err != nil {
			return nil, err
		}
	}
	return inputConsumables, nil
}

func (b *Builder) BuildEssence() (*ledgerstate.TransactionEssence, error) {
	inputConsumables, err := b.addReminderOutput()
	if err != nil {
		return nil, err
	}
	// NewOutputs sorts the outputs and changes indices -> impossible to know index of a particular output
	//outputs := ledgerstate.NewOutputs(b.outputs...)
	outputs := ledgerstate.NewOutputs(b.outputs...)
	inputs := MakeUTXOInputs(inputConsumables...)
	ret := ledgerstate.NewTransactionEssence(b.version, b.timestamp, b.accessPledgeID, b.consensusPledgeID, inputs, outputs)
	return ret, nil
}

func (b *Builder) BuildWithED25519(keyPair *ed25519.KeyPair) (*ledgerstate.Transaction, error) {
	essence, err := b.BuildEssence()
	if err != nil {
		return nil, err
	}
	data := essence.Bytes()
	signature := ledgerstate.NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(data))
	if !signature.AddressSignatureValid(b.senderAddress, data) {
		panic("BuildWithED25519: internal error, signature invalid")
	}
	unlockBlocks := unlockBlocksFromSignature(signature, len(essence.Inputs()))
	return ledgerstate.NewTransaction(essence, unlockBlocks), nil
}

func unlockBlocksFromSignature(signature ledgerstate.Signature, n int) ledgerstate.UnlockBlocks {
	ret := make(ledgerstate.UnlockBlocks, n)
	ret[0] = ledgerstate.NewSignatureUnlockBlock(signature)
	for i := 1; i < n; i++ {
		ret[i] = ledgerstate.NewReferenceUnlockBlock(0)
	}
	return ret
}
