package commitment

import (
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"
)

type MerkleRoot [blake2b.Size256]byte

func NewMerkleRoot(bytes []byte) MerkleRoot {
	b := [blake2b.Size256]byte{}
	copy(b[:], bytes[:])
	return b
}

func (m MerkleRoot) Base58() string {
	return base58.Encode(m[:])
}

func (m MerkleRoot) Bytes() []byte {
	return m[:]
}

/*

// ECRecord is a storable object represents the ecRecord of an epoch.
type ECRecord struct {
	model.Storable[epoch.Index, ECRecord, *ECRecord, ecRecord] `serix:"0"`
}

type ecRecord struct {
	EI     epoch.Index `serix:"0"`
	ECR    RootsID     `serix:"1"`
	PrevID ID          `serix:"2"`
	Roots  *Roots      `serix:"3"`
}

// NewECRecord creates and returns a ECRecord of the given Index.
func NewECRecord(ei epoch.Index) (new *ECRecord) {
	new = model.NewStorable[epoch.Index, ECRecord](&ecRecord{
		EI:     ei,
		ECR:    MerkleRoot{},
		PrevID: MerkleRoot{},
		Roots:  &Roots{},
	})
	new.SetID(ei)
	return
}

func (e *ECRecord) EI() epoch.Index {
	e.RLock()
	defer e.RUnlock()

	return e.M.EI
}

func (e *ECRecord) SetEI(ei epoch.Index) {
	e.Lock()
	defer e.Unlock()

	e.M.EI = ei
	e.SetID(ei)

	e.SetModified()
}

// RootsID returns the RootsID of an ECRecord.
func (e *ECRecord) ECR() RootsID {
	e.RLock()
	defer e.RUnlock()

	return e.M.ECR
}

// SetECR sets the RootsID of an ECRecord.
func (e *ECRecord) SetECR(ecr RootsID) {
	e.Lock()
	defer e.Unlock()

	e.M.ECR = NewMerkleRoot(ecr[:])
	e.SetModified()
}

// PrevID returns the ID of an ECRecord.
func (e *ECRecord) PrevID() ID {
	e.RLock()
	defer e.RUnlock()

	return e.M.PrevID
}

// SetPrevEC sets the PrevID of an ECRecord.
func (e *ECRecord) SetPrevEC(prevEC ID) {
	e.Lock()
	defer e.Unlock()

	e.M.PrevID = NewMerkleRoot(prevEC[:])
	e.SetModified()
}

func (e *ECRecord) Bytes() (bytes []byte, err error) {
	bytes, err = e.Storable.Bytes()
	return
}

func (e *ECRecord) FromBytes(bytes []byte) (err error) {
	_, err = e.Storable.FromBytes(bytes)
	e.SetID(e.EI())

	return
}

// Roots returns the Roots of an ECRecord.
func (e *ECRecord) Roots() *Roots {
	e.RLock()
	defer e.RUnlock()

	return e.M.Roots
}

// PublishRoots sets the Roots of an ECRecord.
func (e *ECRecord) PublishRoots(roots *Roots) {
	e.Lock()
	defer e.Unlock()

	e.M.Roots = roots
	e.SetModified()
}

// ComputeEC calculates the epoch commitment hash from the given ECRecord.
func (e *ECRecord) ComputeEC() (ec ID) {
	ecHash := blake2b.Sum256(byteutils.ConcatBytes(e.EI().Bytes(), e.ECR().Bytes(), e.PrevID().Bytes()))

	return NewMerkleRoot(ecHash[:])
}

*/
