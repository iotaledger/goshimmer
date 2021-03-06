package ledgerstate

import (
	"bytes"
	"fmt"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"golang.org/x/crypto/blake2b"
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
	balances      ColoredBalances

	// alias id is immutable after created
	aliasId      AliasID
	aliasIdMutex sync.RWMutex //TODO is it necessary ?

	// address which controls the state and state metadata if != nil
	// alias destroy command is == nil
	// It can only be changed by governing entity, if set. Otherwise it is self governed.
	stateAddress Address
	// optional state metadata. nil means it is absent
	stateMetadata []byte
	// governance parameter if set
	governanceParams GovernanceParams

	objectstorage.StorableObjectFlags
}

type GovernanceParams struct {
	Set               bool
	GovernedByAddress bool
	GoverningAddress  Address
	GoverningAlias    AliasID
	Matadata          []byte
}

const MaxMetadataSize = 8 * 1024

// flags use to compress serialized bytes
const (
	flagAliasOutputMint                      = 0x01
	flagAliasOutputDestroy                   = 0x02
	flagAliasOutputGovernanceSet             = 0x03
	flagAliasOutputGovernedByAddress         = 0x08
	flagAliasOutputStateMetadataPresent      = 0x10
	flagAliasOutputGovernanceMetadataPresent = 0x20
)

// NewAliasOutputMint creates new AliasOutput as minting output.
func NewAliasOutputMint(balances ColoredBalances, stateAddr Address, stateMetadata []byte, governanceParams ...GovernanceParams) (*AliasOutput, error) {
	ret := &AliasOutput{
		mintNew:       true,
		balances:      balances,
		stateAddress:  stateAddr,
		stateMetadata: stateMetadata,
	}
	if len(stateMetadata) > MaxMetadataSize {
		return nil, xerrors.New("metadata too big")
	}
	if len(governanceParams) > 0 {
		if len(governanceParams[0].Matadata) > MaxMetadataSize {
			return nil, xerrors.New("metadata too big")
		}
		ret.governanceParams = governanceParams[0]
	}
	ret.mustConsistent()
	return ret, nil
}

// NewAliasOutputDestroy creates new AliasOutput for destroy
func (a *AliasOutput) NewAliasOutputDestroy() *AliasOutput {
	ret := &AliasOutput{
		destroy:      true,
		aliasId:      a.aliasId,
		stateAddress: a.stateAddress.Clone(),
	}
	ret.mustConsistent()
	return ret
}

// NewAliasOutputStateTransition creates new AliasOutput as state transition from the previous
func (a *AliasOutput) NewAliasOutputStateTransition(balances ColoredBalances, stateMetadata []byte) *AliasOutput {
	a.mustConsistent()
	ret := &AliasOutput{
		balances:         *balances.Clone(),
		aliasId:          a.GetAliasID(),
		stateAddress:     a.stateAddress.Clone(),
		stateMetadata:    make([]byte, len(stateMetadata)),
		governanceParams: a.governanceParams.clone(),
	}
	copy(ret.stateMetadata, stateMetadata)
	ret.mustConsistent()
	return ret
}

// NewAliasOutputUpdateGovernance creates new output from previous by updating governance parameters
func (a *AliasOutput) NewAliasOutputUpdateGovernance(stateAddress Address, params ...GovernanceParams) *AliasOutput {
	a.mustConsistent()
	ret := a.clone()
	a.stateAddress = stateAddress
	if len(params) > 0 {
		ret.governanceParams = params[0]
	}
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
		balances:         *a.balances.Clone(),
		aliasId:          a.aliasId,
		stateAddress:     a.stateAddress.Clone(),
		stateMetadata:    make([]byte, len(a.stateMetadata)),
		governanceParams: a.governanceParams.clone(),
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
func (a *AliasOutput) GetAliasID() AliasID {
	a.aliasIdMutex.Lock()
	defer a.aliasIdMutex.Unlock()

	if a.mintNew {
		return blake2b.Sum256(a.ID().Bytes())
	}
	return a.aliasId
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
	ret += fmt.Sprintf("   stateMetadataSize: %d\n", len(a.stateMetadata))
	ret += a.governanceParams.String()
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

func (p *GovernanceParams) clone() GovernanceParams {
	ret := GovernanceParams{}
	if !p.Set {
		return ret
	}
	ret.GovernedByAddress = p.GovernedByAddress
	if p.GovernedByAddress {
		ret.GoverningAddress = p.GoverningAddress.Clone()
	} else {
		ret.GoverningAlias = p.GoverningAlias
	}
	if p.Matadata != nil {
		ret.Matadata = make([]byte, len(p.Matadata))
		copy(ret.Matadata, p.Matadata)
	}
	return ret
}

func (p *GovernanceParams) String() string {
	if !p.Set {
		return fmt.Sprintf("Governance: self-governed")
	}
	if p.GovernedByAddress {
		return fmt.Sprintf("Governing address: %s metadata size: %d", p.GoverningAddress.String(), len(p.Matadata))
	}
	return fmt.Sprintf("Governing alias: %s metadata size: %d", p.GoverningAlias.String(), len(p.Matadata))
}

func (a *AliasOutput) checkConsistency() error {
	if a.stateAddress == nil {
		return xerrors.New("AliasOutput: inconsistency 1")
	}
	if a.mintNew && a.destroy {
		return xerrors.New("AliasOutput: inconsistency 2")
	}
	if !a.destroy {
		if a.balances.Size() == 0 {
			// TODO minimum deposit
			return xerrors.New("AliasOutput: inconsistency 3")
		}
	}
	if a.destroy {
		if a.Balances() != nil {
			return xerrors.New("AliasOutput: inconsistency 4")
		}
	}
	if a.governanceParams.Set {
		if a.governanceParams.GovernedByAddress {
			if a.governanceParams.GoverningAddress == nil {
				return xerrors.New("AliasOutput: inconsistency 5")
			}
		} else {
			if a.governanceParams.GoverningAlias == AliasNil {
				return xerrors.New("AliasOutput: inconsistency 6")
			}
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
	if a.governanceParams.Set {
		ret |= flagAliasOutputGovernanceSet
		if a.governanceParams.GovernedByAddress {
			ret |= flagAliasOutputGovernedByAddress
		}
		if a.governanceParams.Matadata != nil {
			ret |= flagAliasOutputGovernanceMetadataPresent
		}
	}
	return ret
}

func (a *AliasOutput) ObjectStorageValue() []byte {
	flags := a.mustFlags()
	ret := marshalutil.New().
		WriteByte(byte(AliasOutputType)).
		WriteByte(flags)
	if flags&flagAliasOutputMint == 0 {
		ret.WriteBytes(a.aliasId.Bytes())
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
	if flags&flagAliasOutputGovernedByAddress != 0 {
		ret.WriteBytes(a.governanceParams.GoverningAddress.Bytes())
	} else {
		ret.WriteBytes(a.governanceParams.GoverningAlias.Bytes())
	}
	if flags&flagAliasOutputGovernanceMetadataPresent != 0 {
		ret.WriteUint16(uint16(len(a.governanceParams.Matadata))).
			WriteBytes(a.governanceParams.Matadata)
	}
	return ret.Bytes()
}
