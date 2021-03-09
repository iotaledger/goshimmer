package ledgerstate

import (
	"bytes"
	"fmt"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"golang.org/x/xerrors"
	"sync"
)

const MinimumIOTAOnAlias = uint64(100) // TODO protocol wide dust threshold

// code contract (make sure the type implements all required methods)
var _ Output = &AliasOutput{}

// AliasOutput represents output which defines an alias and AliasAddress
type AliasOutput struct {
	// common for all outputs
	outputId      OutputID
	outputIdMutex sync.RWMutex
	balances      ColoredBalances

	// alias id becomes immutable after created for a lifetime
	aliasAddress AliasAddress

	// address which controls the state and state metadata if != nil
	// alias destroy command is == nil
	// It can only be changed by governing entity, if set. Otherwise it is self governed.
	// It should be an address unlocked by signature, not alias
	stateAddress Address
	// optional state metadata. nil means it is absent
	stateData []byte
	// if the AliasOutput is a chained, the flags states if it is updating state or governance data.
	// unlock validation of the corresponding input depends on it
	isStateUpdate bool
	// governance address if set. It can be any address, unlocked by signature of alias
	governingAddress Address

	objectstorage.StorableObjectFlags
}

// MaxOutputPayloadSize limit to put on the payload.
const MaxOutputPayloadSize = 4 * 1024

// flags use to compress serialized bytes
const (
	flagAliasOutputStateUpdate      = 0x01
	flagAliasOutputGovernanceSet    = 0x02
	flagAliasOutputStateDataPresent = 0x04
)

// NewAliasOutputMint creates new AliasOutput as minting output, i.e. the one which does not contain corresponding input.
func NewAliasOutputMint(balances map[Color]uint64, stateAddr Address) (*AliasOutput, error) {
	if len(balances) == 0 {
		return nil, xerrors.New("colored balances should not be empty")
	}
	if stateAddr == nil || stateAddr.Type() == AliasAddressType {
		return nil, xerrors.New("mandatory state address must be backed by a private key")
	}
	ret := &AliasOutput{
		balances:     *NewColoredBalances(balances),
		stateAddress: stateAddr,
	}
	if err := ret.checkValidity(); err != nil {
		return nil, err
	}
	return ret, nil
}

// NewAliasOutputStateTransition creates new AliasOutput as state transition from the previous one
func (a *AliasOutput) NewAliasOutputStateTransition(stateUpdate bool) *AliasOutput {
	ret := a.clone()
	ret.isStateUpdate = stateUpdate
	return ret
}

// AliasOutputFromMarshalUtil unmarshals a AliasOutput using a MarshalUtil (for easier unmarshaling).
func AliasOutputFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (*AliasOutput, error) {
	var ret *AliasOutput
	outputType, err := marshalUtil.ReadByte()
	if err != nil {
		return nil, xerrors.Errorf("failed to parse OutputType (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	if OutputType(outputType) != AliasOutputType {
		return nil, xerrors.Errorf("invalid OutputType (%X): %w", outputType, cerrors.ErrParseBytesFailed)
	}
	ret = &AliasOutput{}
	flags, err := marshalUtil.ReadByte()
	if err != nil {
		return nil, xerrors.Errorf("failed to parse AliasOutput flags (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	ret.isStateUpdate = flags&flagAliasOutputStateUpdate != 0
	if ret.aliasAddress.IsMint() {
		addr, err := AliasAddressFromMarshalUtil(marshalUtil)
		if err != nil {
			return nil, xerrors.Errorf("failed to parse alias address (%v): %w", err, cerrors.ErrParseBytesFailed)
		}
		ret.aliasAddress = *addr
	}
	cb, err := ColoredBalancesFromMarshalUtil(marshalUtil)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse colored balances: %w", err)
	}
	ret.balances = *cb
	ret.stateAddress, err = AddressFromMarshalUtil(marshalUtil)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse state address (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	if flags&flagAliasOutputStateDataPresent != 0 {
		size, err := marshalUtil.ReadUint16()
		if err != nil {
			return nil, xerrors.Errorf("failed to parse state data size: %w", err)
		}
		ret.stateData, err = marshalUtil.ReadBytes(int(size))
		if err != nil {
			return nil, xerrors.Errorf("failed to parse state data: %w", err)
		}
	}
	if flags&flagAliasOutputGovernanceSet != 0 {
		ret.governingAddress, err = AddressFromMarshalUtil(marshalUtil)
		if err != nil {
			return nil, xerrors.Errorf("failed to parse governing address (%v): %w", err, cerrors.ErrParseBytesFailed)
		}
	}
	if err := ret.checkValidity(); err != nil {
		return nil, err
	}
	return ret, nil
}

// Clone clones the structure
func (a *AliasOutput) Clone() Output {
	return a.clone()
}

func (a *AliasOutput) clone() *AliasOutput {
	a.mustValidate()
	ret := &AliasOutput{
		outputId:     a.outputId,
		balances:     *a.balances.Clone(),
		aliasAddress: a.aliasAddress,
		stateAddress: a.stateAddress.Clone(),
		stateData:    make([]byte, len(a.stateData)),
	}
	if a.governingAddress != nil {
		ret.governingAddress = a.governingAddress.Clone()
	}
	copy(ret.stateData, a.stateData)
	ret.mustValidate()
	return ret
}

func (a *AliasOutput) SetBalances(bals map[Color]uint64) error {
	if len(bals) == 0 {
		return xerrors.New("colored balances should not be empty")
	}
	if iotas, ok := bals[ColorIOTA]; !ok || iotas < MinimumIOTAOnAlias {
		return xerrors.New("balances are less than dust threshold")
	}
	a.balances = *NewColoredBalances(bals)
	return nil
}

// GetAliasID calculates new ID if it is a minting output. Otherwise it takes stored value
func (a *AliasOutput) GetAliasAddress() *AliasAddress {
	if a.aliasAddress.IsMint() {
		return NewAliasAddress(a.ID().Bytes())
	}
	return &a.aliasAddress
}

func (a *AliasOutput) IsSelfGoverned() bool {
	return a.governingAddress == nil
}

func (a *AliasOutput) SetStateAddress(addr Address) error {
	if addr == nil || addr.Type() == AliasAddressType {
		return xerrors.New("mandatory state address must by backed by a private key")
	}
	a.stateAddress = addr
	return nil
}

func (a *AliasOutput) SetGoverningAddress(addr Address) {
	if addr.Array() == a.stateAddress.Array() {
		addr = nil // self governing
	}
	a.governingAddress = addr
}

func (a *AliasOutput) GetGoverningAddress() Address {
	if a.IsSelfGoverned() {
		return a.stateAddress
	}
	return a.governingAddress
}

func (a *AliasOutput) SetStateData(data []byte) error {
	if len(data) > MaxOutputPayloadSize {
		return xerrors.New("state data too big")
	}
	a.stateData = make([]byte, len(data))
	copy(a.stateData, data)
	return nil
}

func (a *AliasOutput) GetStateData() []byte {
	return a.stateData
}

func (a *AliasOutput) ID() OutputID {
	a.outputIdMutex.RLock()
	defer a.outputIdMutex.RUnlock()

	return a.outputId
}

func (a *AliasOutput) SetID(outputID OutputID) Output {
	a.outputIdMutex.Lock()
	defer a.outputIdMutex.Unlock()

	a.outputId = outputID
	return a
}

func (a *AliasOutput) Type() OutputType {
	return AliasOutputType
}

func (a *AliasOutput) Balances() *ColoredBalances {
	return &a.balances
}

func (a *AliasOutput) Address() Address {
	return a.stateAddress
}

func (a *AliasOutput) Input() Input {
	if a.ID() == EmptyOutputID {
		panic("Outputs that haven't been assigned an ID, yet cannot be converted to an Input")
	}

	return NewUTXOInput(a.ID())
}

func (a *AliasOutput) Bytes() []byte {
	return a.ObjectStorageValue()
}

func (a *AliasOutput) String() string {
	ret := "AliasOutput:\n"
	ret += fmt.Sprintf("   outputId: %s\n", a.ID())
	ret += fmt.Sprintf("   balance: %s\n", a.balances.String())
	ret += fmt.Sprintf("   stateAddress: %s\n", a.stateAddress)
	ret += fmt.Sprintf("   stateMetadataSize: %d\n", len(a.stateData))
	ret += fmt.Sprintf("   governingAddress (self-governed=%v): %s\n", a.IsSelfGoverned(), a.GetGoverningAddress())
	return ret
}

func (a *AliasOutput) Compare(other Output) int {
	return bytes.Compare(a.Bytes(), other.Bytes())
}

func (a *AliasOutput) Update(other objectstorage.StorableObject) {
	panic("AliasOutput: storage object updates disabled")
}

func (a *AliasOutput) ObjectStorageKey() []byte {
	return a.ID().Bytes()
}

func (a *AliasOutput) checkValidity() error {
	if len(a.balances.Map()) == 0 {
		return xerrors.New("balances must not be nil")
	}
	if iotas, ok := a.balances.Get(ColorIOTA); !ok || iotas < MinimumIOTAOnAlias {
		return xerrors.New("balances are less than dust threshold")
	}
	if a.stateAddress == nil {
		return xerrors.New("state address must not be nil")
	}
	if len(a.stateData) > MaxOutputPayloadSize {
		return xerrors.New("state data too big")
	}
	return nil
}

func (a *AliasOutput) mustValidate() {
	if err := a.checkValidity(); err != nil {
		panic(err)
	}
}

func (a *AliasOutput) mustFlags() byte {
	a.mustValidate()
	var ret byte
	if a.isStateUpdate {
		ret |= flagAliasOutputStateUpdate
	}
	if len(a.stateData) > 0 {
		ret |= flagAliasOutputStateDataPresent
	}
	if a.governingAddress != nil {
		ret |= flagAliasOutputGovernanceSet
	}
	return ret
}

func (a *AliasOutput) ObjectStorageValue() []byte {
	flags := a.mustFlags()
	ret := marshalutil.New().
		WriteByte(byte(AliasOutputType)).
		WriteByte(flags).
		WriteBytes(a.aliasAddress.Bytes()).
		WriteBytes(a.balances.Bytes()).
		WriteBytes(a.stateAddress.Bytes())
	if flags&flagAliasOutputStateDataPresent != 0 {
		ret.WriteUint16(uint16(len(a.stateData))).
			WriteBytes(a.stateData)
	}
	if flags&flagAliasOutputGovernanceSet != 0 {
		ret.WriteBytes(a.governingAddress.Bytes())
	}
	return ret.Bytes()
}

// UnlockValid check unlock and validates chain
func (a *AliasOutput) UnlockValid(tx *Transaction, unlockBlock UnlockBlock, inputs []Output) (bool, error) {
	switch blk := unlockBlock.(type) {
	case *SignatureUnlockBlock:
		stateUnlocked, governanceUnlocked, err := a.unlockedBySignature(tx, blk)

		return stateUnlocked || governanceUnlocked, err

	case *AliasReferencedUnlockBlock:
		// state cannot be unlocked by alias reference
		return a.unlockedGovernanceByAliasIndex(tx, blk.ReferencedIndex(), inputs)
	}
	return false, xerrors.New("unsupported unlock block type")
}

func (a *AliasOutput) findChainedOutput(tx *Transaction) (*AliasOutput, error) {
	var ret *AliasOutput
	aliasAddress := a.GetAliasAddress()
	for _, out := range tx.Essence().Outputs() {
		if out.Type() != AliasOutputType {
			continue
		}
		outAlias := out.(*AliasOutput)
		if !aliasAddress.Equal(outAlias.GetAliasAddress()) {
			continue
		}
		if ret != nil {
			return nil, xerrors.Errorf("duplicated alias output: %s", aliasAddress.String())
		}
		ret = outAlias
	}
	return ret, nil
}

// unlockedBySig only possible unlocked for both state and governance when self-governed
func (a *AliasOutput) unlockedBySig(tx *Transaction, sigBlock *SignatureUnlockBlock) (bool, bool) {
	stateSigValid := sigBlock.signature.AddressSignatureValid(a.stateAddress, tx.Essence().Bytes())
	if a.IsSelfGoverned() {
		return stateSigValid, stateSigValid
	}
	if stateSigValid {
		return true, false
	}
	return false, sigBlock.signature.AddressSignatureValid(a.governingAddress, tx.Essence().Bytes())
}

func equalColoredBalance(b1, b2 ColoredBalances) bool {
	allColors := make(map[Color]bool)
	b1.ForEach(func(col Color, bal uint64) bool {
		allColors[col] = true
		return true
	})
	b2.ForEach(func(col Color, bal uint64) bool {
		allColors[col] = true
		return true
	})
	for col := range allColors {
		v1, ok1 := b1.Get(col)
		v2, ok2 := b2.Get(col)
		if ok1 != ok2 || v1 != v2 {
			return false
		}
	}
	return true
}

func isDust(b ColoredBalances) bool {
	bal, ok := b.Get(ColorIOTA)
	if !ok || bal < MinimumIOTAOnAlias {
		return true
	}
	return false
}

func isExactMinimum(b ColoredBalances) bool {
	bals := b.Map()
	if len(bals) != 1 {
		return false
	}
	bal, ok := bals[ColorIOTA]
	if !ok || bal != MinimumIOTAOnAlias {
		return false
	}
	return true
}

func (a *AliasOutput) validateStateTransition(chained *AliasOutput, unlockedState bool) error {
	if unlockedState {
		if !chained.isStateUpdate {
			return xerrors.New("wrong unlock for state update")
		}
		if isDust(chained.balances) {
			return xerrors.New("tokens are below dust threshold")
		}
	}
	// should not modify state data
	if !bytes.Equal(a.stateData, chained.stateData) {
		return xerrors.New("state data is locked for modification")
	}
	if !equalColoredBalance(a.balances, chained.balances) {
		return xerrors.New("tokens are locked for modification")
	}
	return nil
}

func (a *AliasOutput) validateGovernanceChange(chained *AliasOutput, unlockedGovernance bool) error {
	if unlockedGovernance {
		if chained.isStateUpdate {
			return xerrors.New("wrong unlock for governance change")
		}
		return nil
	}
	// must not modify governance data
	if bytes.Compare(a.stateAddress.Bytes(), chained.stateAddress.Bytes()) != 0 {
		return xerrors.New("state address is locked for modification")
	}
	if !a.IsSelfGoverned() {
		if bytes.Compare(a.governingAddress.Bytes(), chained.governingAddress.Bytes()) != 0 {
			return xerrors.New("state address is locked for modification")
		}
	}
	return nil
}

func (a *AliasOutput) validateDestroyTransition(unlockedGovernance bool) error {
	if !unlockedGovernance {
		return xerrors.New("didn't find chained output and alias is not unlocked to be destroyed.")
	}
	if !isExactMinimum(a.balances) {
		return xerrors.New("didn't find chained output and there are more tokens then upper limit for alias destruction")
	}
	return nil
}

func (a *AliasOutput) validateTransition(tx *Transaction, unlockedState, unlockedGovernance bool) error {
	chained, err := a.findChainedOutput(tx)
	if err != nil {
		return err
	}
	if chained != nil {
		if err := a.validateStateTransition(chained, unlockedState); err != nil {
			return err
		}
		if err := a.validateGovernanceChange(chained, unlockedGovernance); err != nil {
			return err
		}
	} else {
		// no chained output found. Alias is being destroyed?
		if err := a.validateDestroyTransition(unlockedGovernance); err != nil {
			return err
		}
	}
	return nil
}

func (a *AliasOutput) unlockedBySignature(tx *Transaction, sigBlock *SignatureUnlockBlock) (bool, bool, error) {
	unlockedState, unlockedGovernance := a.unlockedBySig(tx, sigBlock)
	if !unlockedState && !unlockedGovernance {
		return false, false, nil
	}
	if err := a.validateTransition(tx, unlockedState, unlockedGovernance); err != nil {
		return false, false, nil
	}
	return unlockedState, unlockedGovernance, nil
}

// unlockedGovernanceByAliasIndex unlock one step of alias dereference
func (a *AliasOutput) unlockedGovernanceByAliasIndex(tx *Transaction, refIndex uint16, inputs []Output) (bool, error) {
	if a.IsSelfGoverned() {
		return false, xerrors.New("self-governing alias can be unlocked only by signature")
	}
	if a.governingAddress.Type() != AliasAddressType {
		return false, xerrors.New("expected governing address of AliasAddress type")
	}
	if int(refIndex) > len(inputs) {
		return false, xerrors.New("wrong reference index")
	}
	refInput, ok := inputs[refIndex].(*AliasOutput)
	if !ok {
		return false, xerrors.New("the referenced output is not of AliasOutput type")
	}
	if !refInput.GetAliasAddress().Equal(a.governingAddress.(*AliasAddress)) {
		return false, xerrors.New("wrong alias reference")
	}
	return refInput.IsInputUnlockedForStateUpdate(tx), nil
}

func (a *AliasOutput) IsInputUnlockedForStateUpdate(tx *Transaction) bool {
	chained, err := a.findChainedOutput(tx)
	if err != nil {
		return false
	}
	return chained.isStateUpdate
}
