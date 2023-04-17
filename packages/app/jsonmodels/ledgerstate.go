package jsonmodels

import (
	"encoding/json"
	"time"

	"github.com/mr-tron/base58"
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/typeutils"
	"github.com/iotaledger/hive.go/lo"
)

// region Address //////////////////////////////////////////////////////////////////////////////////////////////////////

// Address represents the JSON model of a ledgerstate.Address.
type Address struct {
	Type   string `json:"type"`
	Base58 string `json:"base58"`
}

// NewAddress returns an Address from the given ledgerstate.Address.
func NewAddress(address devnetvm.Address) *Address {
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
func NewOutput(output devnetvm.Output) (result *Output) {
	return &Output{
		OutputID: NewOutputID(output.ID()),
		Type:     output.Type().String(),
		Output:   MarshalOutput(output),
	}
}

// ToLedgerstateOutput converts the json output object into a goshimmer representation.
func (o *Output) ToLedgerstateOutput() (devnetvm.Output, error) {
	outputType, err := devnetvm.OutputTypeFromString(o.Type)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse output type")
	}
	var id utxo.OutputID
	if iErr := id.FromBase58(o.OutputID.Base58); iErr != nil {
		return nil, errors.Wrap(iErr, "failed to parse outputID")
	}

	switch outputType {
	case devnetvm.SigLockedSingleOutputType:
		s, uErr := UnmarshalSigLockedSingleOutputFromBytes(o.Output)
		if uErr != nil {
			return nil, uErr
		}
		res, tErr := s.ToLedgerStateOutput(id)
		if tErr != nil {
			return nil, tErr
		}
		return res, nil
	case devnetvm.SigLockedColoredOutputType:
		s, uErr := UnmarshalSigLockedColoredOutputFromBytes(o.Output)
		if uErr != nil {
			return nil, uErr
		}
		res, tErr := s.ToLedgerStateOutput(id)
		if tErr != nil {
			return nil, tErr
		}
		return res, nil
	case devnetvm.AliasOutputType:
		s, uErr := UnmarshalAliasOutputFromBytes(o.Output)
		if uErr != nil {
			return nil, uErr
		}
		res, tErr := s.ToLedgerStateOutput(id)
		if tErr != nil {
			return nil, tErr
		}
		return res, nil

	case devnetvm.ExtendedLockedOutputType:
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
func MarshalOutput(output devnetvm.Output) []byte {
	var res interface{}
	switch output.Type() {
	case devnetvm.SigLockedSingleOutputType:
		var err error
		res, err = SigLockedSingleOutputFromLedgerstate(output)
		if err != nil {
			return nil
		}
	case devnetvm.SigLockedColoredOutputType:
		var err error
		res, err = SigLockedColoredOutputFromLedgerstate(output)
		if err != nil {
			return nil
		}
	case devnetvm.AliasOutputType:
		var err error
		res, err = AliasOutputFromLedgerstate(output)
		if err != nil {
			return nil
		}
	case devnetvm.ExtendedLockedOutputType:
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
func (s *SigLockedSingleOutput) ToLedgerStateOutput(id utxo.OutputID) (devnetvm.Output, error) {
	addy, err := devnetvm.AddressFromBase58EncodedString(s.Address)
	if err != nil {
		return nil, errors.Wrap(err, "wrong address in SigLockedSingleOutput")
	}
	res := devnetvm.NewSigLockedSingleOutput(s.Balance, addy)
	res.SetID(id)
	return res, nil
}

// SigLockedSingleOutputFromLedgerstate creates a JSON compatible representation of a ledgerstate output.
func SigLockedSingleOutputFromLedgerstate(output devnetvm.Output) (*SigLockedSingleOutput, error) {
	if output.Type() != devnetvm.SigLockedSingleOutputType {
		return nil, errors.Errorf("wrong output type: %s", output.Type().String())
	}
	balance, _ := output.Balances().Get(devnetvm.ColorIOTA)
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
		return nil, errors.Wrap(err, "failed to unmarshal SigLockedSingleOutput")
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
func (s *SigLockedColoredOutput) ToLedgerStateOutput(id utxo.OutputID) (devnetvm.Output, error) {
	addy, err := devnetvm.AddressFromBase58EncodedString(s.Address)
	if err != nil {
		return nil, errors.Wrap(err, "wrong address in SigLockedSingleOutput")
	}
	balances, bErr := getColoredBalances(s.Balances)
	if bErr != nil {
		return nil, errors.Wrap(bErr, "failed to parse colored balances")
	}

	res := devnetvm.NewSigLockedColoredOutput(balances, addy)
	res.SetID(id)
	return res, nil
}

// SigLockedColoredOutputFromLedgerstate creates a JSON compatible representation of a ledgerstate output.
func SigLockedColoredOutputFromLedgerstate(output devnetvm.Output) (*SigLockedColoredOutput, error) {
	if output.Type() != devnetvm.SigLockedColoredOutputType {
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
		return nil, errors.Wrap(err, "failed to unmarshal SigLockedSingleOutput")
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
func (a *AliasOutput) ToLedgerStateOutput(id utxo.OutputID) (devnetvm.Output, error) {
	balances, err := getColoredBalances(a.Balances)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse colored balances")
	}
	// alias address
	aliasAddy, aErr := devnetvm.AliasAddressFromBase58EncodedString(a.AliasAddress)
	if aErr != nil {
		return nil, errors.Wrap(aErr, "wrong alias address in AliasOutput")
	}
	// state address
	stateAddy, aErr := devnetvm.AddressFromBase58EncodedString(a.StateAddress)
	if aErr != nil {
		return nil, errors.Wrap(aErr, "wrong state address in AliasOutput")
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
	res := &devnetvm.AliasOutput{}
	res.SetID(id)
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
		addy, gErr := devnetvm.AddressFromBase58EncodedString(a.GoverningAddress)
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
func AliasOutputFromLedgerstate(output devnetvm.Output) (*AliasOutput, error) {
	if output.Type() != devnetvm.AliasOutputType {
		return nil, errors.Errorf("wrong output type: %s", output.Type().String())
	}
	castedOutput := output.(*devnetvm.AliasOutput)
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
		return nil, errors.Wrap(err, "failed to unmarshal AliasOutput")
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
func (e *ExtendedLockedOutput) ToLedgerStateOutput(id utxo.OutputID) (devnetvm.Output, error) {
	addy, err := devnetvm.AddressFromBase58EncodedString(e.Address)
	if err != nil {
		return nil, errors.Wrap(err, "wrong address in ExtendedLockedOutput")
	}
	balances, bErr := getColoredBalances(e.Balances)
	if bErr != nil {
		return nil, errors.Wrap(bErr, "failed to parse colored balances")
	}

	res := devnetvm.NewExtendedLockedOutput(balances.Map(), addy)

	if e.FallbackAddress != "" && e.FallbackDeadline != 0 {
		fallbackAddy, fErr := devnetvm.AddressFromBase58EncodedString(e.FallbackAddress)
		if fErr != nil {
			return nil, errors.Wrap(fErr, "wrong fallback address in ExtendedLockedOutput")
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
func ExtendedLockedOutputFromLedgerstate(output devnetvm.Output) (*ExtendedLockedOutput, error) {
	if output.Type() != devnetvm.ExtendedLockedOutputType {
		return nil, errors.Errorf("wrong output type: %s", output.Type().String())
	}
	res := &ExtendedLockedOutput{
		Address:  output.Address().Base58(),
		Balances: getStringBalances(output),
	}
	castedOutput := output.(*devnetvm.ExtendedLockedOutput)
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
		return nil, errors.Wrap(err, "failed to unmarshal ExtendedLockedOutput")
	}
	return marshalledOutput, nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputID /////////////////////////////////////////////////////////////////////////////////////////////////////

// OutputID represents the JSON model of a ledgerstate.OutputID.
type OutputID struct {
	Base58 string `json:"base58"`
}

// NewOutputID returns an OutputID from the given ledgerstate.OutputID.
func NewOutputID(outputID utxo.OutputID) *OutputID {
	return &OutputID{
		Base58: outputID.Base58(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputMetadata ///////////////////////////////////////////////////////////////////////////////////////////////

// OutputMetadata represents the JSON model of the mempool.OutputMetadata.
type OutputMetadata struct {
	OutputID              *OutputID          `json:"outputID"`
	ConflictIDs           []string           `json:"conflictIDs"`
	FirstConsumer         string             `json:"firstCount"`
	ConfirmedConsumer     string             `json:"confirmedConsumer,omitempty"`
	ConfirmationState     confirmation.State `json:"confirmationState"`
	ConfirmationStateTime int64              `json:"confirmationStateTime"`
}

// NewOutputMetadata returns the OutputMetadata from the given mempool.OutputMetadata.
func NewOutputMetadata(outputMetadata *mempool.OutputMetadata, confirmedConsumerID utxo.TransactionID) *OutputMetadata {
	return &OutputMetadata{
		OutputID: NewOutputID(outputMetadata.ID()),
		ConflictIDs: lo.Map(lo.Map(outputMetadata.ConflictIDs().Slice(), func(t utxo.TransactionID) []byte {
			return lo.PanicOnErr(t.Bytes())
		}), base58.Encode),
		FirstConsumer:         outputMetadata.FirstConsumer().Base58(),
		ConfirmedConsumer:     confirmedConsumerID.Base58(),
		ConfirmationState:     outputMetadata.ConfirmationState(),
		ConfirmationStateTime: outputMetadata.ConfirmationStateTime().Unix(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Consumer /////////////////////////////////////////////////////////////////////////////////////////////////////

// Consumer represents the JSON model of a mempool.Consumer.
type Consumer struct {
	TransactionID string `json:"transactionID"`
	Booked        bool   `json:"booked"`
}

// NewConsumer returns a Consumer from the given mempool.Consumer.
func NewConsumer(consumer *mempool.Consumer) *Consumer {
	return &Consumer{
		TransactionID: consumer.TransactionID().Base58(),
		Booked:        consumer.IsBooked(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// ConflictWeight represents the JSON model of a ledger.Conflict.
type ConflictWeight struct {
	ID                string             `json:"id"`
	Parents           []string           `json:"parents"`
	ConflictIDs       []string           `json:"conflictIDs,omitempty"`
	ConfirmationState confirmation.State `json:"confirmationState"`
	ApprovalWeight    int64              `json:"approvalWeight"`
}

// NewConflictWeight returns a Conflict from the given ledger.Conflict.
func NewConflictWeight(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID], confirmationState confirmation.State, aw int64) ConflictWeight {
	return ConflictWeight{
		ID: conflict.ID().Base58(),
		Parents: func() []string {
			parents := make([]string, 0)
			for it := conflict.Parents().Iterator(); it.HasNext(); {
				parents = append(parents, it.Next().Base58())
			}

			return parents
		}(),
		ConflictIDs: func() []string {
			conflictIDs := make([]string, 0)
			for it := conflict.ConflictSets().Iterator(); it.HasNext(); {
				conflictIDs = append(conflictIDs, it.Next().ID().Base58())
			}

			return conflictIDs
		}(),
		ConfirmationState: confirmationState,
		ApprovalWeight:    aw,
	}
}

// region ChildConflict //////////////////////////////////////////////////////////////////////////////////////////////////

// ChildConflict represents the JSON model of a ledger.ChildConflict.
type ChildConflict struct {
	ConflictID string `json:"conflictID"`
}

// NewChildConflict returns a ChildConflict from the given ledger.ChildConflict.
func NewChildConflict(childConflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) *ChildConflict {
	return &ChildConflict{
		ConflictID: childConflict.ID().Base58(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Conflict /////////////////////////////////////////////////////////////////////////////////////////////////////

// Conflict represents the JSON model of a ledger.Conflict.
type Conflict struct {
	OutputID    *OutputID `json:"outputID"`
	ConflictIDs []string  `json:"conflictIDs"`
}

// NewConflict returns a Conflict from the given ledger.ConflictID.
func NewConflict(conflictID utxo.OutputID, conflictIDs []utxo.TransactionID) *Conflict {
	return &Conflict{
		OutputID: NewOutputID(conflictID),
		ConflictIDs: func() (mappedConflictIDs []string) {
			mappedConflictIDs = make([]string, 0)
			for _, conflictID := range conflictIDs {
				mappedConflictIDs = append(mappedConflictIDs, conflictID.Base58())
			}

			return
		}(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Transaction //////////////////////////////////////////////////////////////////////////////////////////////////

// Transaction represents the JSON model of a ledgerstate.Transaction.
type Transaction struct {
	Version           devnetvm.TransactionEssenceVersion `json:"version"`
	Timestamp         int64                              `json:"timestamp"`
	AccessPledgeID    string                             `json:"accessPledgeID"`
	ConsensusPledgeID string                             `json:"consensusPledgeID"`
	Inputs            []*Input                           `json:"inputs"`
	Outputs           []*Output                          `json:"outputs"`
	UnlockBlocks      []*UnlockBlock                     `json:"unlockBlocks"`
	DataPayload       []byte                             `json:"dataPayload"`
}

// NewTransaction returns a Transaction from the given ledgerstate.Transaction.
func NewTransaction(transaction *devnetvm.Transaction) *Transaction {
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
		dataPayload = lo.PanicOnErr(transaction.Essence().Payload().Bytes())
	}

	return &Transaction{
		Version:           transaction.Essence().Version(),
		Timestamp:         transaction.Essence().Timestamp().Unix(),
		AccessPledgeID:    base58.Encode(lo.PanicOnErr(transaction.Essence().AccessPledgeID().Bytes())),
		ConsensusPledgeID: base58.Encode(lo.PanicOnErr(transaction.Essence().ConsensusPledgeID().Bytes())),
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
func NewInput(input devnetvm.Input, referencedOutput ...*Output) *Input {
	if input.Type() == devnetvm.UTXOInputType {
		if len(referencedOutput) < 1 {
			return &Input{
				Type:               input.Type().String(),
				ReferencedOutputID: NewOutputID(input.(*devnetvm.UTXOInput).ReferencedOutputID()),
			}
		}
		return &Input{
			Type:               input.Type().String(),
			ReferencedOutputID: NewOutputID(input.(*devnetvm.UTXOInput).ReferencedOutputID()),
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
	Type            string                 `json:"type"`
	ReferencedIndex uint16                 `json:"referencedIndex,omitempty"`
	SignatureType   devnetvm.SignatureType `json:"signatureType,omitempty"`
	PublicKey       string                 `json:"publicKey,omitempty"`
	Signature       string                 `json:"signature,omitempty"`
}

// NewUnlockBlock returns an UnlockBlock from the given ledgerstate.UnlockBlock.
func NewUnlockBlock(unlockBlock devnetvm.UnlockBlock) *UnlockBlock {
	result := &UnlockBlock{
		Type: unlockBlock.Type().String(),
	}

	switch unlockBlock.Type() {
	case devnetvm.SignatureUnlockBlockType:
		signature, _, _ := devnetvm.SignatureFromBytes(lo.PanicOnErr(unlockBlock.Bytes()))
		result.SignatureType = signature.Type()
		switch signature.Type() {
		case devnetvm.ED25519SignatureType:
			signature, _, _ := devnetvm.ED25519SignatureFromBytes(signature.Bytes())
			result.PublicKey = signature.PublicKey.String()
			result.Signature = signature.Signature.String()

		case devnetvm.BLSSignatureType:
			signature, _, _ := devnetvm.BLSSignatureFromBytes(signature.Bytes())
			result.Signature = signature.Signature.String()
		}
	case devnetvm.ReferenceUnlockBlockType:
		referenceUnlockBlock, _, _ := devnetvm.ReferenceUnlockBlockFromBytes(lo.PanicOnErr(unlockBlock.Bytes()))
		result.ReferencedIndex = referenceUnlockBlock.ReferencedIndex()
	}

	return result
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionMetadata ///////////////////////////////////////////////////////////////////////////////////////////

// TransactionMetadata represents the JSON model of the mempool.TransactionMetadata.
type TransactionMetadata struct {
	TransactionID         string             `json:"transactionID"`
	ConflictIDs           []string           `json:"conflictIDs"`
	Booked                bool               `json:"booked"`
	BookedTime            int64              `json:"bookedTime"`
	ConfirmationState     confirmation.State `json:"confirmationState"`
	ConfirmationStateTime int64              `json:"confirmationStateTime"`
}

// NewTransactionMetadata returns the TransactionMetadata from the given mempool.TransactionMetadata.
func NewTransactionMetadata(transactionMetadata *mempool.TransactionMetadata) *TransactionMetadata {
	return &TransactionMetadata{
		TransactionID:         transactionMetadata.ID().Base58(),
		ConflictIDs:           lo.Map(lo.Map(transactionMetadata.ConflictIDs().Slice(), func(t utxo.TransactionID) []byte { return lo.PanicOnErr(t.Bytes()) }), base58.Encode),
		Booked:                transactionMetadata.IsBooked(),
		BookedTime:            transactionMetadata.BookingTime().Unix(),
		ConfirmationState:     transactionMetadata.ConfirmationState(),
		ConfirmationStateTime: transactionMetadata.ConfirmationStateTime().Unix(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region utils ////////////////////////////////////////////////////////////////////////////////////////////////////////

// getStringBalances translates colored balances to map[string]uint64.
func getStringBalances(output devnetvm.Output) map[string]uint64 {
	balances := output.Balances().Map()
	stringBalances := make(map[string]uint64, len(balances))
	for color, balance := range balances {
		stringBalances[color.Base58()] = balance
	}
	return stringBalances
}

// getColoredBalances translates a map[string]uint64 to ledgerstate.ColoredBalances.
func getColoredBalances(stringBalances map[string]uint64) (*devnetvm.ColoredBalances, error) {
	cBalances := make(map[devnetvm.Color]uint64, len(stringBalances))
	for stringColor, balance := range stringBalances {
		color, cErr := devnetvm.ColorFromBase58EncodedString(stringColor)
		if cErr != nil {
			return nil, errors.Wrap(cErr, "failed to decode color")
		}
		cBalances[color] = balance
	}
	return devnetvm.NewColoredBalances(cBalances), nil
}

// endregion
