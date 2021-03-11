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

const DustThresholdChainOutputIOTA = uint64(100) // TODO protocol wide dust threshold

// code contract (make sure the type implements all required methods)
var _ Output = &ChainOutput{}

// ChainOutput represents output which defines as AliasAddress.
// It can only be used in a chained manner
type ChainOutput struct {
	// common for all outputs
	outputId      OutputID
	outputIdMutex sync.RWMutex
	balances      ColoredBalances

	// aliasAddress becomes immutable after created for a lifetime. It is returned as Address()
	aliasAddress AliasAddress

	// address which controls the state and state data
	// It can only be changed by governing entity, if set. Otherwise it is self governed.
	// It should be an address unlocked by signature, not AliasAddress
	stateAddress Address
	// optional state metadata. nil means it is absent
	stateData []byte
	// if the ChainOutput is chained in the transaction, the flags states if it is updating state or governance data.
	// unlock validation of the corresponding input depends on it.
	// The flag is used to prevent a need to check signature each time when checking unlocking mode
	isGovernanceUpdate bool
	// governance address if set. It can be any address, unlocked by signature of alias address. Nil means self governed
	governingAddress Address

	objectstorage.StorableObjectFlags
}

// MaxOutputPayloadSize limit to put on the payload.
const MaxOutputPayloadSize = 4 * 1024

// flags use to compress serialized bytes
const (
	flagChainOutputGovernanceUpdate = 0x01
	flagChainOutputGovernanceSet    = 0x02
	flagChainOutputStateDataPresent = 0x04
)

// NewChainOutputMint creates new ChainOutput as minting output, i.e. the one which does not contain corresponding input.
func NewChainOutputMint(balances map[Color]uint64, stateAddr Address) (*ChainOutput, error) {
	if !IsAboveDustThreshold(balances) {
		return nil, xerrors.New("ChainOutput: colored balances should not be empty")
	}
	if stateAddr == nil || stateAddr.Type() == AliasAddressType {
		return nil, xerrors.New("ChainOutput: mandatory state address must be backed by a private key")
	}
	ret := &ChainOutput{
		balances:     *NewColoredBalances(balances),
		stateAddress: stateAddr,
	}
	if err := ret.checkValidity(); err != nil {
		return nil, err
	}
	return ret, nil
}

// NewChainOutputNext creates new ChainOutput as state transition from the previous one
func (c *ChainOutput) NewChainOutputNext(governanceUpdate ...bool) *ChainOutput {
	ret := c.clone()
	ret.aliasAddress = *c.GetAliasAddress()
	ret.isGovernanceUpdate = false
	if len(governanceUpdate) > 0 {
		ret.isGovernanceUpdate = governanceUpdate[0]
	}
	return ret
}

// ChainOutputFromMarshalUtil unmarshals a ChainOutput using a MarshalUtil (for easier unmarshaling).
func ChainOutputFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (*ChainOutput, error) {
	var ret *ChainOutput
	outputType, err := marshalUtil.ReadByte()
	if err != nil {
		return nil, xerrors.Errorf("ChainOutput: failed to parse OutputType (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	if OutputType(outputType) != ChainOutputType {
		return nil, xerrors.Errorf("ChainOutput: invalid OutputType (%X): %w", outputType, cerrors.ErrParseBytesFailed)
	}
	ret = &ChainOutput{}
	flags, err := marshalUtil.ReadByte()
	if err != nil {
		return nil, xerrors.Errorf("ChainOutput: failed to parse ChainOutput flags (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	ret.isGovernanceUpdate = flags&flagChainOutputGovernanceUpdate != 0
	if ret.aliasAddress.IsMint() {
		addr, err := AliasAddressFromMarshalUtil(marshalUtil)
		if err != nil {
			return nil, xerrors.Errorf("ChainOutput: failed to parse alias address (%v): %w", err, cerrors.ErrParseBytesFailed)
		}
		ret.aliasAddress = *addr
	}
	cb, err := ColoredBalancesFromMarshalUtil(marshalUtil)
	if err != nil {
		return nil, xerrors.Errorf("ChainOutput: failed to parse colored balances: %w", err)
	}
	ret.balances = *cb
	ret.stateAddress, err = AddressFromMarshalUtil(marshalUtil)
	if err != nil {
		return nil, xerrors.Errorf("ChainOutput: failed to parse state address (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	if flags&flagChainOutputStateDataPresent != 0 {
		size, err := marshalUtil.ReadUint16()
		if err != nil {
			return nil, xerrors.Errorf("ChainOutput: failed to parse state data size: %w", err)
		}
		ret.stateData, err = marshalUtil.ReadBytes(int(size))
		if err != nil {
			return nil, xerrors.Errorf("ChainOutput: failed to parse state data: %w", err)
		}
	}
	if flags&flagChainOutputGovernanceSet != 0 {
		ret.governingAddress, err = AddressFromMarshalUtil(marshalUtil)
		if err != nil {
			return nil, xerrors.Errorf("ChainOutput: failed to parse governing address (%v): %w", err, cerrors.ErrParseBytesFailed)
		}
	}
	if err := ret.checkValidity(); err != nil {
		return nil, err
	}
	return ret, nil
}

// Clone clones the structure
func (c *ChainOutput) Clone() Output {
	return c.clone()
}

func (c *ChainOutput) clone() *ChainOutput {
	c.mustValidate()
	ret := &ChainOutput{
		outputId:     c.outputId,
		balances:     *c.balances.Clone(),
		aliasAddress: c.aliasAddress,
		stateAddress: c.stateAddress.Clone(),
		stateData:    make([]byte, len(c.stateData)),
	}
	if c.governingAddress != nil {
		ret.governingAddress = c.governingAddress.Clone()
	}
	copy(ret.stateData, c.stateData)
	ret.mustValidate()
	return ret
}

func (c *ChainOutput) SetBalances(balances map[Color]uint64) error {
	if !IsAboveDustThreshold(balances) {
		return xerrors.New("ChainOutput: balances are less than dust threshold")
	}
	c.balances = *NewColoredBalances(balances)
	return nil
}

// GetAliasAddress calculates new ID if it is a minting output. Otherwise it takes stored value
func (c *ChainOutput) GetAliasAddress() *AliasAddress {
	if c.aliasAddress.IsMint() {
		return NewAliasAddress(c.ID().Bytes())
	}
	return &c.aliasAddress
}

func (c *ChainOutput) IsSelfGoverned() bool {
	return c.governingAddress == nil
}

func (c *ChainOutput) GetStateAddress() Address {
	return c.stateAddress
}

func (c *ChainOutput) SetStateAddress(addr Address) error {
	if addr == nil || addr.Type() == AliasAddressType {
		return xerrors.New("ChainOutput: mandatory state address cannot be c AliasAddress")
	}
	c.stateAddress = addr
	return nil
}

func (c *ChainOutput) SetGoverningAddress(addr Address) {
	if addr.Array() == c.stateAddress.Array() {
		addr = nil // self governing
	}
	c.governingAddress = addr
}

func (c *ChainOutput) GetGoverningAddress() Address {
	if c.IsSelfGoverned() {
		return c.stateAddress
	}
	return c.governingAddress
}

func (c *ChainOutput) SetStateData(data []byte) error {
	if len(data) > MaxOutputPayloadSize {
		return xerrors.New("ChainOutput: state data too big")
	}
	c.stateData = make([]byte, len(data))
	copy(c.stateData, data)
	return nil
}

func (c *ChainOutput) GetStateData() []byte {
	return c.stateData
}

func (c *ChainOutput) ID() OutputID {
	c.outputIdMutex.RLock()
	defer c.outputIdMutex.RUnlock()

	return c.outputId
}

func (c *ChainOutput) SetID(outputID OutputID) Output {
	c.outputIdMutex.Lock()
	defer c.outputIdMutex.Unlock()

	c.outputId = outputID
	return c
}

func (c *ChainOutput) Type() OutputType {
	return ChainOutputType
}

func (c *ChainOutput) Balances() *ColoredBalances {
	return &c.balances
}

// Address ChainOutput is searchable in the ledger through its AliasAddress
func (c *ChainOutput) Address() Address {
	return c.GetAliasAddress()
}

func (c *ChainOutput) Input() Input {
	if c.ID() == EmptyOutputID {
		panic("ChainOutput: Outputs that haven't been assigned an ID, yet cannot be converted to an Input")
	}

	return NewUTXOInput(c.ID())
}

func (c *ChainOutput) Bytes() []byte {
	return c.ObjectStorageValue()
}

func (c *ChainOutput) String() string {
	ret := "ChainOutput:\n"
	ret += fmt.Sprintf("   address: %s\n", c.Address())
	ret += fmt.Sprintf("   outputId: %s\n", c.ID())
	ret += fmt.Sprintf("   balance: %s\n", c.balances.String())
	ret += fmt.Sprintf("   stateAddress: %s\n", c.stateAddress)
	ret += fmt.Sprintf("   stateMetadataSize: %d\n", len(c.stateData))
	ret += fmt.Sprintf("   governingAddress (self-governed=%v): %s\n", c.IsSelfGoverned(), c.GetGoverningAddress())
	return ret
}

func (c *ChainOutput) Compare(other Output) int {
	return bytes.Compare(c.Bytes(), other.Bytes())
}

func (c *ChainOutput) Update(other objectstorage.StorableObject) {
	panic("ChainOutput: storage object updates disabled")
}

func (c *ChainOutput) ObjectStorageKey() []byte {
	return c.ID().Bytes()
}

func (c *ChainOutput) checkValidity() error {
	if !IsAboveDustThreshold(c.balances) {
		return xerrors.New("ChainOutput: balances are below dust threshold")
	}
	if c.stateAddress == nil {
		return xerrors.New("ChainOutput: state address must not be nil")
	}
	if len(c.stateData) > MaxOutputPayloadSize {
		return xerrors.New("ChainOutput: state data too big")
	}
	return nil
}

func (c *ChainOutput) mustValidate() {
	if err := c.checkValidity(); err != nil {
		panic(err)
	}
}

func (c *ChainOutput) mustFlags() byte {
	c.mustValidate()
	var ret byte
	if c.isGovernanceUpdate {
		ret |= flagChainOutputGovernanceUpdate
	}
	if len(c.stateData) > 0 {
		ret |= flagChainOutputStateDataPresent
	}
	if c.governingAddress != nil {
		ret |= flagChainOutputGovernanceSet
	}
	return ret
}

func (c *ChainOutput) ObjectStorageValue() []byte {
	flags := c.mustFlags()
	ret := marshalutil.New().
		WriteByte(byte(ChainOutputType)).
		WriteByte(flags).
		WriteBytes(c.aliasAddress.Bytes()).
		WriteBytes(c.balances.Bytes()).
		WriteBytes(c.stateAddress.Bytes())
	if flags&flagChainOutputStateDataPresent != 0 {
		ret.WriteUint16(uint16(len(c.stateData))).
			WriteBytes(c.stateData)
	}
	if flags&flagChainOutputGovernanceSet != 0 {
		ret.WriteBytes(c.governingAddress.Bytes())
	}
	return ret.Bytes()
}

// UnlockValid check unlock and validates chain
func (c *ChainOutput) UnlockValid(tx *Transaction, unlockBlock UnlockBlock, inputs []Output) (bool, error) {
	switch blk := unlockBlock.(type) {
	case *SignatureUnlockBlock:
		stateUnlocked, governanceUnlocked, err := c.unlockedBySignature(tx, blk)

		return stateUnlocked || governanceUnlocked, err

	case *AliasReferencedUnlockBlock:
		// state cannot be unlocked by alias reference, so only checking governance mode
		return c.unlockedGovernanceByAliasIndex(tx, blk.ReferencedIndex(), inputs)
	}
	return false, xerrors.New("unsupported unlock block type")
}

func (c *ChainOutput) findChainedOutput(tx *Transaction) (*ChainOutput, error) {
	var ret *ChainOutput
	aliasAddress := c.GetAliasAddress()
	for _, out := range tx.Essence().Outputs() {
		if out.Type() != ChainOutputType {
			continue
		}
		outAlias := out.(*ChainOutput)
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
func (c *ChainOutput) unlockedBySig(tx *Transaction, sigBlock *SignatureUnlockBlock) (bool, bool) {
	stateSigValid := sigBlock.signature.AddressSignatureValid(c.stateAddress, tx.Essence().Bytes())
	if c.IsSelfGoverned() {
		return stateSigValid, stateSigValid
	}
	if stateSigValid {
		return true, false
	}
	return false, sigBlock.signature.AddressSignatureValid(c.governingAddress, tx.Essence().Bytes())
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

func IsAboveDustThreshold(b interface{}) bool {
	switch m := b.(type) {
	case ColoredBalances:
		bal, ok := m.Get(ColorIOTA)
		if !ok || bal >= DustThresholdChainOutputIOTA {
			return true
		}
	case map[Color]uint64:
		if iotas, ok := m[ColorIOTA]; !ok || iotas >= DustThresholdChainOutputIOTA {
			return true
		}
	default:
		panic("wrong parameter type in IsAboveDustThreshold")
	}
	return false
}

func isExactMinimum(b ColoredBalances) bool {
	bals := b.Map()
	if len(bals) != 1 {
		return false
	}
	bal, ok := bals[ColorIOTA]
	if !ok || bal != DustThresholdChainOutputIOTA {
		return false
	}
	return true
}

// validateStateTransition checks if part controlled by state address is valid
func (c *ChainOutput) validateStateTransition(chained *ChainOutput, unlockedState bool) error {
	if unlockedState {
		if chained.isGovernanceUpdate {
			return xerrors.New("ChainOutput: wrong unlock for state update")
		}
		if !IsAboveDustThreshold(chained.balances) {
			return xerrors.New("ChainOutput: tokens are below dust threshold")
		}
		return nil
	}
	// locked: should not modify state data nor tokens
	if !bytes.Equal(c.stateData, chained.stateData) {
		return xerrors.New("ChainOutput: state data is locked for modification")
	}
	if !equalColoredBalance(c.balances, chained.balances) {
		return xerrors.New("ChainOutput: tokens are locked for modification")
	}

	return nil
}

// validateGovernanceChange checks if the parte controlled by the governing party is valid
func (c *ChainOutput) validateGovernanceChange(chained *ChainOutput, unlockedGovernance bool) error {
	if unlockedGovernance {
		return nil
	}
	if c.isGovernanceUpdate {
		return xerrors.New("ChainOutput: wrong governance update flag")
	}
	// locked: must not modify governance data
	if bytes.Compare(c.stateAddress.Bytes(), chained.stateAddress.Bytes()) != 0 {
		return xerrors.New("ChainOutput: state address is locked for modification")
	}
	if c.IsSelfGoverned() != chained.IsSelfGoverned() {
		return xerrors.New("ChainOutput: governing address is locked for modification")
	}
	if !c.IsSelfGoverned() {
		if bytes.Compare(c.governingAddress.Bytes(), chained.governingAddress.Bytes()) != 0 {
			return xerrors.New("ChainOutput: governing address is locked for modification")
		}
	}
	return nil
}

// validateDestroyTransition check validity if input is not chained (destroyed)
func (c *ChainOutput) validateDestroyTransition(unlockedGovernance bool) error {
	if !unlockedGovernance {
		return xerrors.New("ChainOutput: didn't find chained output and alias is not unlocked to be destroyed.")
	}
	if !isExactMinimum(c.balances) {
		return xerrors.New("ChainOutput: didn't find chained output and there are more tokens then upper limit for alias destruction")
	}
	return nil
}

func (c *ChainOutput) validateTransition(tx *Transaction, unlockedState, unlockedGovernance bool) error {
	chained, err := c.findChainedOutput(tx)
	if err != nil {
		return err
	}

	if chained != nil {
		if !c.GetAliasAddress().Equal(c.GetAliasAddress()) {
			return xerrors.New("chain alias address can't be modified")
		}
		if err := c.validateStateTransition(chained, unlockedState); err != nil {
			return err
		}
		if err := c.validateGovernanceChange(chained, unlockedGovernance); err != nil {
			return err
		}
	} else {
		// no chained output found. Alias is being destroyed?
		if err := c.validateDestroyTransition(unlockedGovernance); err != nil {
			return err
		}
	}
	return nil
}

func (c *ChainOutput) unlockedBySignature(tx *Transaction, sigBlock *SignatureUnlockBlock) (bool, bool, error) {
	unlockedState, unlockedGovernance := c.unlockedBySig(tx, sigBlock)
	if !unlockedState && !unlockedGovernance {
		return false, false, nil
	}
	if err := c.validateTransition(tx, unlockedState, unlockedGovernance); err != nil {
		return false, false, nil
	}
	return unlockedState, unlockedGovernance, nil
}

// unlockedGovernanceByAliasIndex unlock one step of alias dereference
func (c *ChainOutput) unlockedGovernanceByAliasIndex(tx *Transaction, refIndex uint16, inputs []Output) (bool, error) {
	if c.IsSelfGoverned() {
		return false, xerrors.New("ChainOutput: self-governing alias output can't be unlocked by alias reference")
	}
	if c.governingAddress.Type() != AliasAddressType {
		return false, xerrors.New("ChainOutput: expected governing address of AliasAddress type")
	}
	if int(refIndex) > len(inputs) {
		return false, xerrors.New("ChainOutput: wrong alias reference index")
	}
	refInput, ok := inputs[refIndex].(*ChainOutput)
	if !ok {
		return false, xerrors.New("ChainOutput: the referenced output is not of ChainOutput type")
	}
	if !refInput.GetAliasAddress().Equal(c.governingAddress.(*AliasAddress)) {
		return false, xerrors.New("ChainOutput: wrong alias reference address")
	}
	return !refInput.IsUnlockedForGovernanceUpdate(tx), nil
}

func (c *ChainOutput) IsUnlockedForGovernanceUpdate(tx *Transaction) bool {
	chained, err := c.findChainedOutput(tx)
	if err != nil {
		return false
	}
	return chained.isGovernanceUpdate
}
