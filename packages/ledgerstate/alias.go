package ledgerstate

import (
	"bytes"
	"fmt"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"golang.org/x/xerrors"
	"sync"
)

// code contract (make sure the type implements all required methods)
var _ Output = &AliasOutput{}

type AliasOutput struct {
	mintNew bool
	destroy bool
	// common for all outputs
	outputId      OutputID
	outputIdMutex sync.RWMutex
	balances      *ColoredBalances

	// alias id is immutable after created
	addressAlias AddressAlias
	aliasIdMutex sync.RWMutex //TODO is it necessary ?

	// address which controls the state and state metadata if != nil
	// alias destroy command is == nil
	// It can only be changed by governing entity, if set. Otherwise it is self governed.
	stateAddress Address
	// optional state metadata. nil means it is absent
	stateMetadata []byte
	// governance parameters if set
	governingAddress Address

	objectstorage.StorableObjectFlags
}

const MaxMetadataSize = 8 * 1024

// flags use to compress serialized bytes
const (
	flagAliasOutputMint                 = 0x01
	flagAliasOutputDestroy              = 0x02
	flagAliasOutputGovernanceSet        = 0x04
	flagAliasOutputStateMetadataPresent = 0x08
)

// NewAliasOutputMint creates new AliasOutput as minting output.
func NewAliasOutputMint(balances *ColoredBalances, stateAddr Address, stateMetadata []byte, governingAddress Address) (*AliasOutput, error) {
	ret := &AliasOutput{
		mintNew:          true,
		balances:         balances,
		stateAddress:     stateAddr,
		stateMetadata:    stateMetadata,
		governingAddress: governingAddress,
	}
	if err := ret.checkConsistency(); err != nil {
		return nil, err
	}
	return ret, nil
}

// NewAliasOutputDestroy creates new AliasOutput for destroy
func (a *AliasOutput) NewAliasOutputDestroy() *AliasOutput {
	ret := &AliasOutput{
		destroy:      true,
		addressAlias: a.addressAlias,
		stateAddress: a.stateAddress.Clone(),
	}
	ret.mustConsistent()
	return ret
}

// NewAliasOutputStateTransition creates new AliasOutput as state transition from the previous
func (a *AliasOutput) NewAliasOutputStateTransition(balances ColoredBalances, stateMetadata []byte) *AliasOutput {
	a.mustConsistent()
	ret := &AliasOutput{
		balances:         balances.Clone(),
		addressAlias:     a.GetAddressAlias(),
		stateAddress:     a.stateAddress.Clone(),
		stateMetadata:    make([]byte, len(stateMetadata)),
		governingAddress: a.governingAddress,
	}
	copy(ret.stateMetadata, stateMetadata)
	ret.mustConsistent()
	return ret
}

// NewAliasOutputUpdateGovernance creates new output from previous by updating governance parameters
func (a *AliasOutput) NewAliasOutputUpdateGovernance(stateAddress Address, governingAddress Address, governanceMetadata []byte) *AliasOutput {
	a.mustConsistent()
	ret := a.clone()
	a.stateAddress = stateAddress
	a.governingAddress = governingAddress
	a.mustConsistent()
	return ret
}

// Clone clones the structure
func (a *AliasOutput) Clone() Output {
	return a.clone()
}

func (a *AliasOutput) clone() *AliasOutput {
	a.mustConsistent()
	ret := &AliasOutput{
		mintNew:          a.mintNew,
		destroy:          a.destroy,
		outputId:         a.outputId,
		balances:         a.balances.Clone(),
		addressAlias:     a.addressAlias,
		stateAddress:     a.stateAddress.Clone(),
		stateMetadata:    make([]byte, len(a.stateMetadata)),
		governingAddress: a.governingAddress.Clone(),
	}
	copy(ret.stateMetadata, a.stateMetadata)
	ret.mustConsistent()
	return ret
}

func (a *AliasOutput) ID() OutputID {
	a.outputIdMutex.RLock()
	defer a.outputIdMutex.RUnlock()

	return a.outputId
}

// GetAliasID calculates new ID if it is a minting output. Otherwise it takes stored value
func (a *AliasOutput) GetAddressAlias() AddressAlias {
	a.aliasIdMutex.Lock()
	defer a.aliasIdMutex.Unlock()

	if a.mintNew {
		return *NewAddressAlias(a.ID().Bytes())
	}
	return a.addressAlias
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
	return a.balances
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
	ret += fmt.Sprintf("   balance: %s\n", a.balances)
	ret += fmt.Sprintf("   stateAddress: %s\n", a.stateAddress)
	ret += fmt.Sprintf("   stateMetadataSize: %d\n", len(a.stateMetadata))
	if a.governingAddress == nil {
		ret += fmt.Sprintf("   governingAddress: self governed\n")
	} else {
		ret += fmt.Sprintf("   governingAddress: %s\n", a.governingAddress)
	}
	return ret
}

func (a *AliasOutput) Compare(other Output) int {
	return bytes.Compare(a.Bytes(), other.Bytes())
}

func (a *AliasOutput) Update(other objectstorage.StorableObject) {
	panic("AliasOutput: updates disabled")
}

func (a *AliasOutput) ObjectStorageKey() []byte {
	return a.ID().Bytes()
}

func (a *AliasOutput) checkConsistency() error {
	if a.stateAddress == nil {
		return xerrors.New("address must not be nil")
	}
	if len(a.stateMetadata) > MaxMetadataSize {
		return xerrors.New("metadata too big")
	}
	if a.mintNew && a.destroy {
		return xerrors.New("AliasOutput: inconsistency 1")
	}
	if !a.destroy {
		if a.balances == nil || a.balances.Size() == 0 {
			// TODO minimum deposit
			return xerrors.New("AliasOutput: inconsistency 2")
		}
	}
	if a.destroy {
		if a.balances != nil {
			return xerrors.New("AliasOutput: inconsistency 3")
		}
	}
	return nil
}

func (a *AliasOutput) mustConsistent() {
	if err := a.checkConsistency(); err != nil {
		panic(err)
	}
}

func (a *AliasOutput) mustFlags() byte {
	a.mustConsistent()
	var ret byte
	if a.mintNew {
		ret |= flagAliasOutputMint
	}
	if a.destroy {
		ret |= flagAliasOutputDestroy
	}
	if a.stateMetadata != nil {
		ret |= flagAliasOutputStateMetadataPresent
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
		WriteByte(flags)
	if flags&flagAliasOutputMint == 0 {
		ret.WriteBytes(a.addressAlias.Bytes())
	}
	ret.WriteBytes(a.stateAddress.Bytes())
	if flags&flagAliasOutputDestroy == 0 {
		ret.WriteBytes(a.balances.Bytes())
	}
	if flags&flagAliasOutputStateMetadataPresent != 0 {
		ret.WriteUint16(uint16(len(a.stateMetadata))).
			WriteBytes(a.stateMetadata)
	}
	if flags&flagAliasOutputGovernanceSet == 0 {
		return ret.Bytes()
	}
	ret.WriteBytes(a.governingAddress.Bytes())
	return ret.Bytes()
}
