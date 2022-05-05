package notarization

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// EpochCommitment is a compressed form of all the information (messages and confirmed value payloads) of a certain epoch.
type EpochCommitment struct {
	ECI ECI
}

// EpochCommitmentFactory manages epoch commitments.
type EpochCommitmentFactory struct {
}

// NewCommitmentFactory returns a new commitment factory.
func NewCommitmentFactory() *EpochCommitmentFactory {
	return &EpochCommitmentFactory{}
}

// InsertECTR inserts msg in the ECTR.
func (f *EpochCommitmentFactory) InsertECTR(eci ECI, msgID tangle.MessageID) {

}

// InsertECSR inserts the outputID in the ECSR.
func (f *EpochCommitmentFactory) InsertECSR(eci ECI, outputID ledgerstate.OutputID) {

}

// InsertECSMR inserts the transaction ID in the ECSMR.
func (f *EpochCommitmentFactory) InsertECSMR(eci ECI, txID ledgerstate.TransactionID) {

}

// RemoveECTR removes the message ID from the ECTR.
func (f *EpochCommitmentFactory) RemoveECTR(eci ECI, msgID tangle.MessageID) {

}

// GetCommitment returns the commitment with the given eci.
func (f *EpochCommitmentFactory) GetCommitment(eci ECI) *EpochCommitment {
	return &EpochCommitment{ECI: eci}
}
