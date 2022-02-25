package jsonmodels

import (
	"encoding/json"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/typeutils"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// region Address //////////////////////////////////////////////////////////////////////////////////////////////////////

// Address represents the JSON model of a ledgerstate.Address.
type Address struct {
	Type   string `json:"type"`
	Base58 string `json:"base58"`
}

// NewAddress returns an Address from the given ledgerstate.Address.
func NewAddress(address ledgerstate.Address) *Address {
	return &Address{
		Type:   address.Type().String(),
		Base58: address.Base58(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Output ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Output represents the JSON model of a ledgerstate.Output.
type Output struct {
	OutputID *OutputID       `json:"outputID,omitempty"`
	Type     string          `json:"type"`
	Output   json.RawMessage `json:"output"`
}

// NewOutput returns an Output from the given ledgerstate.Output.
func NewOutput(output ledgerstate.Output) (result *Output) {
	return &Output{
		OutputID: NewOutputID(output.ID()),
		Type:     output.Type().String(),
		Output:   MarshalOutput(output),
	}
}

// ToLedgerstateOutput converts the json output object into a goshimmer representation.
func (o *Output) ToLedgerstateOutput() (ledgerstate.Output, error) {
	outputType, err := ledgerstate.OutputTypeFromString(o.Type)
	if err != nil {
		return nil, errors.Errorf("failed to parse output type: %w", err)
	}
	id, iErr := ledgerstate.OutputIDFromBase58(o.OutputID.Base58)
	if iErr != nil {
		return nil, errors.Errorf("failed to parse outputID: %w", iErr)
	}

	switch outputType {
	case ledgerstate.SigLockedSingleOutputType:
		s, uErr := UnmarshalSigLockedSingleOutputFromBytes(o.Output)
		if uErr != nil {
			return nil, uErr
		}
		res, tErr := s.ToLedgerStateOutput(id)
		if tErr != nil {
			return nil, tErr
		}
		return res, nil
	case ledgerstate.SigLockedColoredOutputType:
		s, uErr := UnmarshalSigLockedColoredOutputFromBytes(o.Output)
		if uErr != nil {
			return nil, uErr
		}
		res, tErr := s.ToLedgerStateOutput(id)
		if tErr != nil {
			return nil, tErr
		}
		return res, nil
	case ledgerstate.AliasOutputType:
		s, uErr := UnmarshalAliasOutputFromBytes(o.Output)
		if uErr != nil {
			return nil, uErr
		}
		res, tErr := s.ToLedgerStateOutput(id)
		if tErr != nil {
			return nil, tErr
		}
		return res, nil

	case ledgerstate.ExtendedLockedOutputType:
		s, uErr := UnmarshalExtendedLockedOutputFromBytes(o.Output)
		if uErr != nil {
			return nil, uErr
		}
		res, tErr := s.ToLedgerStateOutput(id)
		if tErr != nil {
			return nil, tErr
		}
		return res, nil
	default:
		return nil, errors.Errorf("not supported output type: %d", outputType)
	}
}

// MarshalOutput uses the json marshaller to marshal a ledgerstate.Output into bytes.
func MarshalOutput(output ledgerstate.Output) []byte {
	var res interface{}
	switch output.Type() {
	case ledgerstate.SigLockedSingleOutputType:
		var err error
		res, err = SigLockedSingleOutputFromLedgerstate(output)
		if err != nil {
			return nil
		}
	case ledgerstate.SigLockedColoredOutputType:
		var err error
		res, err = SigLockedColoredOutputFromLedgerstate(output)
		if err != nil {
			return nil
		}
	case ledgerstate.AliasOutputType:
		var err error
		res, err = AliasOutputFromLedgerstate(output)
		if err != nil {
			return nil
		}
	case ledgerstate.ExtendedLockedOutputType:
		var err error
		res, err = ExtendedLockedOutputFromLedgerstate(output)
		if err != nil {
			return nil
		}
	default:
		return nil
	}
	byteResult, mErr := json.Marshal(res)
	if mErr != nil {
		// should never happen
		panic(mErr)
	}
	return byteResult
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SigLockedSingleOutput ////////////////////////////////////////////////////////////////////////////////////////

// SigLockedSingleOutput is the JSON model of a ledgerstate.SigLockedSingleOutput.
type SigLockedSingleOutput struct {
	Balance uint64 `json:"balance"`
	Address string `json:"address"`
}

// ToLedgerStateOutput builds a ledgerstate.Output from SigLockedSingleOutput with the given outputID.
func (s *SigLockedSingleOutput) ToLedgerStateOutput(id ledgerstate.OutputID) (ledgerstate.Output, error) {
	addy, err := ledgerstate.AddressFromBase58EncodedString(s.Address)
	if err != nil {
		return nil, errors.Errorf("wrong address in SigLockedSingleOutput: %w", err)
	}
	res := ledgerstate.NewSigLockedSingleOutput(s.Balance, addy)
	res.SetID(id)
	return res, nil
}

// SigLockedSingleOutputFromLedgerstate creates a JSON compatible representation of a ledgerstate output.
func SigLockedSingleOutputFromLedgerstate(output ledgerstate.Output) (*SigLockedSingleOutput, error) {
	if output.Type() != ledgerstate.SigLockedSingleOutputType {
		return nil, errors.Errorf("wrong output type: %s", output.Type().String())
	}
	balance, _ := output.Balances().Get(ledgerstate.ColorIOTA)
	res := &SigLockedSingleOutput{
		Address: output.Address().Base58(),
		Balance: balance,
	}
	return res, nil
}

// UnmarshalSigLockedSingleOutputFromBytes uses the json unmarshaler to unmarshal data into a SigLockedSingleOutput.
func UnmarshalSigLockedSingleOutputFromBytes(data []byte) (*SigLockedSingleOutput, error) {
	marshalledOutput := &SigLockedSingleOutput{}
	err := json.Unmarshal(data, marshalledOutput)
	if err != nil {
		return nil, errors.Errorf("failed to unmarshal SigLockedSingleOutput: %w", err)
	}
	return marshalledOutput, nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SigLockedColoredOutput ///////////////////////////////////////////////////////////////////////////////////////

// SigLockedColoredOutput is the JSON model of a ledgerstate.SigLockedColoredOutput.
type SigLockedColoredOutput struct {
	Balances map[string]uint64 `json:"balances"`
	Address  string            `json:"address"`
}

// ToLedgerStateOutput builds a ledgerstate.Output from SigLockedSingleOutput with the given outputID.
func (s *SigLockedColoredOutput) ToLedgerStateOutput(id ledgerstate.OutputID) (ledgerstate.Output, error) {
	addy, err := ledgerstate.AddressFromBase58EncodedString(s.Address)
	if err != nil {
		return nil, errors.Errorf("wrong address in SigLockedSingleOutput: %w", err)
	}
	balances, bErr := getColoredBalances(s.Balances)
	if bErr != nil {
		return nil, errors.Errorf("failed to parse colored balances: %w", bErr)
	}

	res := ledgerstate.NewSigLockedColoredOutput(balances, addy)
	res.SetID(id)
	return res, nil
}

// SigLockedColoredOutputFromLedgerstate creates a JSON compatible representation of a ledgerstate output.
func SigLockedColoredOutputFromLedgerstate(output ledgerstate.Output) (*SigLockedColoredOutput, error) {
	if output.Type() != ledgerstate.SigLockedColoredOutputType {
		return nil, errors.Errorf("wrong output type: %s", output.Type().String())
	}
	res := &SigLockedColoredOutput{
		Address:  output.Address().Base58(),
		Balances: getStringBalances(output),
	}
	return res, nil
}

// UnmarshalSigLockedColoredOutputFromBytes uses the json unmarshaler to unmarshal data into a SigLockedColoredOutput.
func UnmarshalSigLockedColoredOutputFromBytes(data []byte) (*SigLockedColoredOutput, error) {
	marshalledOutput := &SigLockedColoredOutput{}
	err := json.Unmarshal(data, marshalledOutput)
	if err != nil {
		return nil, errors.Errorf("failed to unmarshal SigLockedSingleOutput: %w", err)
	}
	return marshalledOutput, nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AliasOutput //////////////////////////////////////////////////////////////////////////////////////////////////

// AliasOutput is the JSON model of a ledgerstate.AliasOutput.
type AliasOutput struct {
	Balances           map[string]uint64 `json:"balances"`
	AliasAddress       string            `json:"aliasAddress"`
	StateAddress       string            `json:"stateAddress"`
	StateIndex         uint32            `json:"stateIndex"`
	IsGovernanceUpdate bool              `json:"isGovernanceUpdate"`
	IsOrigin           bool              `json:"isOrigin"`
	IsDelegated        bool              `json:"isDelegated"`

	// marshaled to base64
	StateData          []byte `json:"stateData,omitempty"`
	GovernanceMetadata []byte `json:"governanceMetadata"`
	ImmutableData      []byte `json:"immutableData,omitempty"`

	GoverningAddress   string `json:"governingAddress,omitempty"`
	DelegationTimelock int64  `json:"delegationTimelock,omitempty"`
}

// ToLedgerStateOutput builds a ledgerstate.Output from SigLockedSingleOutput with the given outputID.
func (a *AliasOutput) ToLedgerStateOutput(id ledgerstate.OutputID) (ledgerstate.Output, error) {
	balances, err := getColoredBalances(a.Balances)
	if err != nil {
		return nil, errors.Errorf("failed to parse colored balances: %w", err)
	}
	// alias address
	aliasAddy, aErr := ledgerstate.AliasAddressFromBase58EncodedString(a.AliasAddress)
	if aErr != nil {
		return nil, errors.Errorf("wrong alias address in AliasOutput: %w", err)
	}
	// state address
	stateAddy, aErr := ledgerstate.AddressFromBase58EncodedString(a.StateAddress)
	if aErr != nil {
		return nil, errors.Errorf("wrong state address in AliasOutput: %w", err)
	}
	// stateIndex
	stateIndex := a.StateIndex
	// isGovernanceUpdate
	isGovernanceUpdate := a.IsGovernanceUpdate
	// isOrigin
	isOrigin := a.IsOrigin
	// isDelegated
	isDelegated := a.IsDelegated

	// no suitable constructor, doing it the manual way
	res := &ledgerstate.AliasOutput{}
	res = res.SetID(id).(*ledgerstate.AliasOutput)
	res.SetAliasAddress(aliasAddy)
	err = res.SetBalances(balances.Map())
	if err != nil {
		return nil, err
	}
	err = res.SetStateAddress(stateAddy)
	if err != nil {
		return nil, err
	}
	res.SetStateIndex(stateIndex)
	res.SetIsGovernanceUpdated(isGovernanceUpdate)
	res.SetIsOrigin(isOrigin)
	res.SetIsDelegated(isDelegated)

	// optional fields
	if a.StateData != nil {
		err = res.SetStateData(a.StateData)
		if err != nil {
			return nil, err
		}
	}
	if a.GovernanceMetadata != nil {
		err = res.SetGovernanceMetadata(a.GovernanceMetadata)
		if err != nil {
			return nil, err
		}
	}
	if a.ImmutableData != nil {
		err = res.SetImmutableData(a.ImmutableData)
		if err != nil {
			return nil, err
		}
	}
	if a.GoverningAddress != "" {
		addy, gErr := ledgerstate.AddressFromBase58EncodedString(a.GoverningAddress)
		if gErr != nil {
			return nil, gErr
		}
		res.SetGoverningAddress(addy)
	}
	if a.DelegationTimelock != 0 {
		err = res.SetDelegationTimelock(time.Unix(a.DelegationTimelock, 0))
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

// AliasOutputFromLedgerstate creates a JSON compatible representation of a ledgerstate output.
func AliasOutputFromLedgerstate(output ledgerstate.Output) (*AliasOutput, error) {
	if output.Type() != ledgerstate.AliasOutputType {
		return nil, errors.Errorf("wrong output type: %s", output.Type().String())
	}
	castedOutput := output.(*ledgerstate.AliasOutput)
	res := &AliasOutput{
		Balances:           getStringBalances(output),
		AliasAddress:       castedOutput.GetAliasAddress().Base58(),
		StateAddress:       castedOutput.GetStateAddress().Base58(),
		StateIndex:         castedOutput.GetStateIndex(),
		IsGovernanceUpdate: castedOutput.GetIsGovernanceUpdated(),
		StateData:          castedOutput.GetStateData(),
		GovernanceMetadata: castedOutput.GetGovernanceMetadata(),
		ImmutableData:      castedOutput.GetImmutableData(),
		IsOrigin:           castedOutput.IsOrigin(),
		IsDelegated:        castedOutput.IsDelegated(),
	}

	if !castedOutput.IsSelfGoverned() {
		res.GoverningAddress = castedOutput.GetGoverningAddress().Base58()
	}
	if !castedOutput.DelegationTimelock().IsZero() {
		res.DelegationTimelock = castedOutput.DelegationTimelock().Unix()
	}
	return res, nil
}

// UnmarshalAliasOutputFromBytes uses the json unmarshaler to unmarshal data into an AliasOutput.
func UnmarshalAliasOutputFromBytes(data []byte) (*AliasOutput, error) {
	marshalledOutput := &AliasOutput{}
	err := json.Unmarshal(data, marshalledOutput)
	if err != nil {
		return nil, errors.Errorf("failed to unmarshal AliasOutput: %w", err)
	}
	return marshalledOutput, nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ExtendedLockOutput ///////////////////////////////////////////////////////////////////////////////////////////

// ExtendedLockedOutput is the JSON model of a ledgerstate.ExtendedLockedOutput.
type ExtendedLockedOutput struct {
	Balances         map[string]uint64 `json:"balances"`
	Address          string            `json:"address"`
	FallbackAddress  string            `json:"fallbackAddress,omitempty"`
	FallbackDeadline int64             `json:"fallbackDeadline,omitempty"`
	TimeLock         int64             `json:"timelock,omitempty"`
	Payload          []byte            `json:"payload,omitempty"`
}

// ToLedgerStateOutput builds a ledgerstate.Output from ExtendedLockedOutput with the given outputID.
func (e *ExtendedLockedOutput) ToLedgerStateOutput(id ledgerstate.OutputID) (ledgerstate.Output, error) {
	addy, err := ledgerstate.AddressFromBase58EncodedString(e.Address)
	if err != nil {
		return nil, errors.Errorf("wrong address in ExtendedLockedOutput: %w", err)
	}
	balances, bErr := getColoredBalances(e.Balances)
	if bErr != nil {
		return nil, errors.Errorf("failed to parse colored balances: %w", bErr)
	}

	res := ledgerstate.NewExtendedLockedOutput(balances.Map(), addy)

	if e.FallbackAddress != "" && e.FallbackDeadline != 0 {
		fallbackAddy, fErr := ledgerstate.AddressFromBase58EncodedString(e.FallbackAddress)
		if fErr != nil {
			return nil, errors.Errorf("wrong fallback address in ExtendedLockedOutput: %w", err)
		}
		res = res.WithFallbackOptions(fallbackAddy, time.Unix(e.FallbackDeadline, 0))
	}
	if e.TimeLock != 0 {
		res = res.WithTimeLock(time.Unix(e.TimeLock, 0))
	}
	if e.Payload != nil {
		rErr := res.SetPayload(e.Payload)
		if rErr != nil {
			return nil, rErr
		}
	}
	res.SetID(id)
	return res, nil
}

// ExtendedLockedOutputFromLedgerstate creates a JSON compatible representation of a ledgerstate output.
func ExtendedLockedOutputFromLedgerstate(output ledgerstate.Output) (*ExtendedLockedOutput, error) {
	if output.Type() != ledgerstate.ExtendedLockedOutputType {
		return nil, errors.Errorf("wrong output type: %s", output.Type().String())
	}
	res := &ExtendedLockedOutput{
		Address:  output.Address().Base58(),
		Balances: getStringBalances(output),
	}
	castedOutput := output.(*ledgerstate.ExtendedLockedOutput)
	fallbackAddy, fallbackDeadline := castedOutput.FallbackOptions()
	if !typeutils.IsInterfaceNil(fallbackAddy) {
		res.FallbackAddress = fallbackAddy.Base58()
		res.FallbackDeadline = fallbackDeadline.Unix()
	}
	if !castedOutput.TimeLock().Equal(time.Time{}) {
		res.TimeLock = castedOutput.TimeLock().Unix()
	}
	return res, nil
}

// UnmarshalExtendedLockedOutputFromBytes uses the json unmarshaler to unmarshal data into an ExtendedLockedOutput.
func UnmarshalExtendedLockedOutputFromBytes(data []byte) (*ExtendedLockedOutput, error) {
	marshalledOutput := &ExtendedLockedOutput{}
	err := json.Unmarshal(data, marshalledOutput)
	if err != nil {
		return nil, errors.Errorf("failed to unmarshal ExtendedLockedOutput: %w", err)
	}
	return marshalledOutput, nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputID /////////////////////////////////////////////////////////////////////////////////////////////////////

// OutputID represents the JSON model of a ledgerstate.OutputID.
type OutputID struct {
	Base58        string `json:"base58"`
	TransactionID string `json:"transactionID"`
	OutputIndex   uint16 `json:"outputIndex"`
}

// NewOutputID returns an OutputID from the given ledgerstate.OutputID.
func NewOutputID(outputID ledgerstate.OutputID) *OutputID {
	return &OutputID{
		Base58:        outputID.Base58(),
		TransactionID: outputID.TransactionID().Base58(),
		OutputIndex:   outputID.OutputIndex(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputMetadata ///////////////////////////////////////////////////////////////////////////////////////////////

// OutputMetadata represents the JSON model of the ledgerstate.OutputMetadata.
type OutputMetadata struct {
	OutputID            *OutputID           `json:"outputID"`
	BranchIDs           []string            `json:"branchIDs"`
	Solid               bool                `json:"solid"`
	SolidificationTime  int64               `json:"solidificationTime"`
	ConsumerCount       int                 `json:"consumerCount"`
	ConfirmedConsumer   string              `json:"confirmedConsumer,omitempty"`
	GradeOfFinality     gof.GradeOfFinality `json:"gradeOfFinality"`
	GradeOfFinalityTime int64               `json:"gradeOfFinalityTime"`
}

// NewOutputMetadata returns the OutputMetadata from the given ledgerstate.OutputMetadata.
func NewOutputMetadata(outputMetadata *ledgerstate.OutputMetadata, confirmedConsumerID ledgerstate.TransactionID) *OutputMetadata {
	return &OutputMetadata{
		OutputID:            NewOutputID(outputMetadata.ID()),
		BranchIDs:           outputMetadata.BranchIDs().Base58(),
		Solid:               outputMetadata.Solid(),
		SolidificationTime:  outputMetadata.SolidificationTime().Unix(),
		ConsumerCount:       outputMetadata.ConsumerCount(),
		ConfirmedConsumer:   confirmedConsumerID.Base58(),
		GradeOfFinality:     outputMetadata.GradeOfFinality(),
		GradeOfFinalityTime: outputMetadata.GradeOfFinalityTime().Unix(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Consumer /////////////////////////////////////////////////////////////////////////////////////////////////////

// Consumer represents the JSON model of a ledgerstate.Consumer.
type Consumer struct {
	TransactionID string `json:"transactionID"`
	Valid         string `json:"valid"`
}

// NewConsumer returns a Consumer from the given ledgerstate.Consumer.
func NewConsumer(consumer *ledgerstate.Consumer) *Consumer {
	return &Consumer{
		TransactionID: consumer.TransactionID().Base58(),
		Valid:         consumer.Valid().String(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Branch ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Branch represents the JSON model of a ledgerstate.Branch.
type Branch struct {
	ID              string              `json:"id"`
	Parents         []string            `json:"parents"`
	ConflictIDs     []string            `json:"conflictIDs,omitempty"`
	GradeOfFinality gof.GradeOfFinality `json:"gradeOfFinality"`
	ApprovalWeight  float64             `json:"approvalWeight"`
}

// NewBranch returns a Branch from the given ledgerstate.Branch.
func NewBranch(branch *ledgerstate.Branch, gradeOfFinality gof.GradeOfFinality, aw float64) Branch {
	return Branch{
		ID: branch.ID().Base58(),
		Parents: func() []string {
			parents := make([]string, 0)
			for id := range branch.Parents() {
				parents = append(parents, id.Base58())
			}

			return parents
		}(),
		ConflictIDs: func() []string {
			conflictIDs := make([]string, 0)
			for conflictID := range branch.Conflicts() {
				conflictIDs = append(conflictIDs, conflictID.Base58())
			}

			return conflictIDs
		}(),
		GradeOfFinality: gradeOfFinality,
		ApprovalWeight:  aw,
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ChildBranch //////////////////////////////////////////////////////////////////////////////////////////////////

// ChildBranch represents the JSON model of a ledgerstate.ChildBranch.
type ChildBranch struct {
	BranchID string `json:"branchID"`
}

// NewChildBranch returns a ChildBranch from the given ledgerstate.ChildBranch.
func NewChildBranch(childBranch *ledgerstate.ChildBranch) *ChildBranch {
	return &ChildBranch{
		BranchID: childBranch.ChildBranchID().Base58(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Conflict /////////////////////////////////////////////////////////////////////////////////////////////////////

// Conflict represents the JSON model of a ledgerstate.Conflict.
type Conflict struct {
	OutputID  *OutputID `json:"outputID"`
	BranchIDs []string  `json:"branchIDs"`
}

// NewConflict returns a Conflict from the given ledgerstate.ConflictID.
func NewConflict(conflictID ledgerstate.ConflictID, branchIDs []ledgerstate.BranchID) *Conflict {
	return &Conflict{
		OutputID: NewOutputID(conflictID.OutputID()),
		BranchIDs: func() (mappedBranchIDs []string) {
			mappedBranchIDs = make([]string, 0)
			for _, branchID := range branchIDs {
				mappedBranchIDs = append(mappedBranchIDs, branchID.Base58())
			}

			return
		}(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Transaction //////////////////////////////////////////////////////////////////////////////////////////////////

// Transaction represents the JSON model of a ledgerstate.Transaction.
type Transaction struct {
	Version           ledgerstate.TransactionEssenceVersion `json:"version"`
	Timestamp         int64                                 `json:"timestamp"`
	AccessPledgeID    string                                `json:"accessPledgeID"`
	ConsensusPledgeID string                                `json:"consensusPledgeID"`
	Inputs            []*Input                              `json:"inputs"`
	Outputs           []*Output                             `json:"outputs"`
	UnlockBlocks      []*UnlockBlock                        `json:"unlockBlocks"`
	DataPayload       []byte                                `json:"dataPayload"`
}

// NewTransaction returns a Transaction from the given ledgerstate.Transaction.
func NewTransaction(transaction *ledgerstate.Transaction) *Transaction {
	inputs := make([]*Input, len(transaction.Essence().Inputs()))
	for i, input := range transaction.Essence().Inputs() {
		inputs[i] = NewInput(input)
	}

	outputs := make([]*Output, len(transaction.Essence().Outputs()))
	for i, output := range transaction.Essence().Outputs() {
		outputs[i] = NewOutput(output)
	}

	unlockBlocks := make([]*UnlockBlock, len(transaction.UnlockBlocks()))
	for i, unlockBlock := range transaction.UnlockBlocks() {
		unlockBlocks[i] = NewUnlockBlock(unlockBlock)
	}

	dataPayload := make([]byte, 0)
	if transaction.Essence().Payload() != nil {
		dataPayload = transaction.Essence().Payload().Bytes()
	}

	return &Transaction{
		Version:           transaction.Essence().Version(),
		Timestamp:         transaction.Essence().Timestamp().Unix(),
		AccessPledgeID:    base58.Encode(transaction.Essence().AccessPledgeID().Bytes()),
		ConsensusPledgeID: base58.Encode(transaction.Essence().ConsensusPledgeID().Bytes()),
		Inputs:            inputs,
		Outputs:           outputs,
		UnlockBlocks:      unlockBlocks,
		DataPayload:       dataPayload,
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Input ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Input represents the JSON model of a ledgerstate.Input.
type Input struct {
	Type               string    `json:"type"`
	ReferencedOutputID *OutputID `json:"referencedOutputID,omitempty"`
	// the referenced output object, omit if not specified
	Output *Output `json:"output,omitempty"`
}

// NewInput returns an Input from the given ledgerstate.Input.
func NewInput(input ledgerstate.Input, referencedOutput ...*Output) *Input {
	if input.Type() == ledgerstate.UTXOInputType {
		if len(referencedOutput) < 1 {
			return &Input{
				Type:               input.Type().String(),
				ReferencedOutputID: NewOutputID(input.(*ledgerstate.UTXOInput).ReferencedOutputID()),
			}
		}
		return &Input{
			Type:               input.Type().String(),
			ReferencedOutputID: NewOutputID(input.(*ledgerstate.UTXOInput).ReferencedOutputID()),
			Output:             referencedOutput[0],
		}
	}

	return &Input{
		Type:               "UnknownInputType",
		ReferencedOutputID: nil,
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region UnlockBlock //////////////////////////////////////////////////////////////////////////////////////////////////

// UnlockBlock represents the JSON model of a ledgerstate.UnlockBlock.
type UnlockBlock struct {
	Type            string                    `json:"type"`
	ReferencedIndex uint16                    `json:"referencedIndex,omitempty"`
	SignatureType   ledgerstate.SignatureType `json:"signatureType,omitempty"`
	PublicKey       string                    `json:"publicKey,omitempty"`
	Signature       string                    `json:"signature,omitempty"`
}

// NewUnlockBlock returns an UnlockBlock from the given ledgerstate.UnlockBlock.
func NewUnlockBlock(unlockBlock ledgerstate.UnlockBlock) *UnlockBlock {
	result := &UnlockBlock{
		Type: unlockBlock.Type().String(),
	}

	switch unlockBlock.Type() {
	case ledgerstate.SignatureUnlockBlockType:
		signature, _, _ := ledgerstate.SignatureFromBytes(unlockBlock.Bytes())
		result.SignatureType = signature.Type()
		switch signature.Type() {
		case ledgerstate.ED25519SignatureType:
			signature, _, _ := ledgerstate.ED25519SignatureFromBytes(signature.Bytes())
			result.PublicKey = signature.PublicKey.String()
			result.Signature = signature.Signature.String()

		case ledgerstate.BLSSignatureType:
			signature, _, _ := ledgerstate.BLSSignatureFromBytes(signature.Bytes())
			result.Signature = signature.Signature.String()
		}
	case ledgerstate.ReferenceUnlockBlockType:
		referenceUnlockBlock, _, _ := ledgerstate.ReferenceUnlockBlockFromBytes(unlockBlock.Bytes())
		result.ReferencedIndex = referenceUnlockBlock.ReferencedIndex()
	}

	return result
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionMetadata ///////////////////////////////////////////////////////////////////////////////////////////

// TransactionMetadata represents the JSON model of the ledgerstate.TransactionMetadata.
type TransactionMetadata struct {
	TransactionID       string              `json:"transactionID"`
	BranchIDs           []string            `json:"branchIDs"`
	Solid               bool                `json:"solid"`
	SolidificationTime  int64               `json:"solidificationTime"`
	LazyBooked          bool                `json:"lazyBooked"`
	GradeOfFinality     gof.GradeOfFinality `json:"gradeOfFinality"`
	GradeOfFinalityTime int64               `json:"gradeOfFinalityTime"`
}

// NewTransactionMetadata returns the TransactionMetadata from the given ledgerstate.TransactionMetadata.
func NewTransactionMetadata(transactionMetadata *ledgerstate.TransactionMetadata) *TransactionMetadata {
	return &TransactionMetadata{
		TransactionID:       transactionMetadata.ID().Base58(),
		BranchIDs:           transactionMetadata.BranchIDs().Base58(),
		Solid:               transactionMetadata.Solid(),
		SolidificationTime:  transactionMetadata.SolidificationTime().Unix(),
		LazyBooked:          transactionMetadata.LazyBooked(),
		GradeOfFinality:     transactionMetadata.GradeOfFinality(),
		GradeOfFinalityTime: transactionMetadata.GradeOfFinalityTime().Unix(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region utils ////////////////////////////////////////////////////////////////////////////////////////////////////////

// getStringBalances translates colored balances to map[string]uint64.
func getStringBalances(output ledgerstate.Output) map[string]uint64 {
	balances := output.Balances().Map()
	stringBalances := make(map[string]uint64, len(balances))
	for color, balance := range balances {
		stringBalances[color.Base58()] = balance
	}
	return stringBalances
}

// getColoredBalances translates a map[string]uint64 to ledgerstate.ColoredBalances.
func getColoredBalances(stringBalances map[string]uint64) (*ledgerstate.ColoredBalances, error) {
	cBalances := make(map[ledgerstate.Color]uint64, len(stringBalances))
	for stringColor, balance := range stringBalances {
		color, cErr := ledgerstate.ColorFromBase58EncodedString(stringColor)
		if cErr != nil {
			return nil, errors.Errorf("failed to decode color: %w", cErr)
		}
		cBalances[color] = balance
	}
	return ledgerstate.NewColoredBalances(cBalances), nil
}

// endregion
