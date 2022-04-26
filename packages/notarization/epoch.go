package notarization

// ECI is the index of an epoch commitment.
type ECI uint64

// Epoch is a time range used to define a bucket of messages.
type Epoch struct {
	ECI        ECI
	Confirmed  bool
	Finalized  bool
	Commitment *EpochCommitment
}
