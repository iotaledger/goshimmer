package sybilprotection

type Attestation struct {
	IssuingTime      time.Time         `serix:"0"`
	CommitmentID     commitment.ID     `serix:"1"`
	BlockContentHash types.Identifier  `serix:"2"`
	Signature        ed25519.Signature `serix:"3"`
}

func NewWeightProofEntry(issuingTime time.Time, commitmentID commitment.ID, blockContentHash types.Identifier, signature ed25519.Signature) *Attestation {
	return &Attestation{
		IssuingTime:      issuingTime,
		CommitmentID:     commitmentID,
		BlockContentHash: blockContentHash,
		Signature:        signature,
	}
}

func (w Attestation) Bytes() (bytes []byte, err error) {
	return serix.DefaultAPI.Encode(context.Background(), w, serix.WithValidation())
}

func (w *Attestation) FromBytes(bytes []byte) (consumedBytes int, err error) {
	return serix.DefaultAPI.Decode(context.Background(), bytes, w, serix.WithValidation())
}
