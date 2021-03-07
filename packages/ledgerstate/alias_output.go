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

	// alias id becomes immutable after created
	aliasAddress AliasAddress

	// address which controls the state and state metadata if != nil
	// alias destroy command is == nil
	// It can only be changed by governing entity, if set. Otherwise it is self governed.
	stateAddress Address
	// optional state metadata. nil means it is absent
	stateData []byte
	// governance address if set
	governingAddress Address

	objectstorage.StorableObjectFlags
}

const MaxStateDataSize = 8 * 1024

// flags use to compress serialized bytes
const (
	flagAliasOutputGovernanceSet    = 0x02
	flagAliasOutputStateDataPresent = 0x04
)

// NewAliasOutputMint creates new AliasOutput as minting output.
func NewAliasOutputMint(balances map[Color]uint64, stateAddr Address, stateMetadata []byte, governingAddress Address) (*AliasOutput, error) {
	if len(balances) == 0 {
		return nil, xerrors.New("colored balances should not be empty")
	}
	ret := &AliasOutput{
		balances:         *NewColoredBalances(balances),
		stateAddress:     stateAddr,
		stateData:        stateMetadata,
		governingAddress: governingAddress,
	}
	if err := ret.checkValidity(); err != nil {
		return nil, err
	}
	return ret, nil
}

// NewAliasOutputStateTransition creates new AliasOutput as state transition from the previous
func (a *AliasOutput) NewAliasOutputStateTransition(balances map[Color]uint64, stateMetadata []byte) (*AliasOutput, error) {
	if len(balances) == 0 {
		return nil, xerrors.New("colored balances should not be empty")
	}
	a.mustValidate()
	ret := &AliasOutput{
		balances:         *NewColoredBalances(balances),
		aliasAddress:     *a.GetAliasAddress(),
		stateAddress:     a.stateAddress.Clone(),
		stateData:        make([]byte, len(stateMetadata)),
		governingAddress: a.governingAddress,
	}
	copy(ret.stateData, stateMetadata)
	if err := ret.checkValidity(); err != nil {
		return nil, err
	}
	return ret, nil
}

// NewAliasOutputUpdateGovernance creates new output from previous by updating governance parameters
func (a *AliasOutput) NewAliasOutputUpdateGovernance(stateAddress Address, governingAddress Address, governanceMetadata []byte) *AliasOutput {
	a.mustValidate()
	ret := a.clone()
	a.stateAddress = stateAddress
	a.governingAddress = governingAddress
	a.mustValidate()
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
		outputId:         a.outputId,
		balances:         *a.balances.Clone(),
		aliasAddress:     a.aliasAddress,
		stateAddress:     a.stateAddress.Clone(),
		stateData:        make([]byte, len(a.stateData)),
		governingAddress: a.governingAddress.Clone(),
	}
	copy(ret.stateData, a.stateData)
	ret.mustValidate()
	return ret
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

func (a *AliasOutput) GetGoverningAddress() Address {
	if a.IsSelfGoverned() {
		return a.stateAddress
	}
	return a.governingAddress
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

func (a *AliasOutput) UnlockValid(tx *Transaction, unlockBlock UnlockBlock) (bool, error) {
	panic("implement me")
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
	if len(a.stateData) > MaxStateDataSize {
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
