package signaturescheme

import (
	"fmt"
	"math/rand"

	"github.com/mr-tron/base58"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing/bn256"
	"go.dedis.ch/kyber/v3/sign"
	"go.dedis.ch/kyber/v3/sign/bdn"
	"go.dedis.ch/kyber/v3/util/random"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
)

// bls.go implements BLS signature scheme which is robust against rogue public key attacks,
// called "Boneh-Drijvers-Neven" or BDN
// It uses go.dedis/kyber library. More info https://github.com/dedis/kyber/blob/master/sign/bdn/bdn.go
// Often BLS signatures are used as threshold signatures.
// This package doesn't implement any threshold signature related primitives.
// it only contains what is needed for the node to check validity of the BLS signatures against addresses,
// signature aggregation function and minimum signing required for testing
var suite = bn256.NewSuite()

const (
	// BLSSignatureSize represents the length in bytes of a BLS signature.
	BLSSignatureSize = 64

	// BLSPublicKeySize represents the length in bytes of a BLS public key.
	BLSPublicKeySize = 128

	// BLSPrivateKeySize represents the length in bytes of a BLS private key.
	BLSPrivateKeySize = 32

	// BLSFullSignatureSize represents the length in bytes of a full BLS signature.
	BLSFullSignatureSize = 1 + BLSPublicKeySize + BLSSignatureSize
)

// ---------------- implements SignatureScheme interface
// blsSignatureScheme defines an interface for the key pairs of BLS signatures.
type blsSignatureScheme struct {
	priKey kyber.Scalar
	pubKey kyber.Point
}

// deterministic sequence
var rnd = random.New(rand.New(rand.NewSource(42)))

// RandBLS creates a RANDOM instance of a signature scheme, that is used to sign the corresponding address.
// only for testing: each time same sequence!
func RandBLS() SignatureScheme {
	ret := &blsSignatureScheme{}
	ret.priKey, ret.pubKey = bdn.NewKeyPair(suite, rnd)
	return ret
}

// BLS creates an instance of BLS signature scheme
// from given private and public keys in marshaled binary form
func BLS(priKey, pubKey []byte) (SignatureScheme, error) {
	if len(priKey) != BLSPrivateKeySize || len(pubKey) != BLSPublicKeySize {
		return nil, fmt.Errorf("wrong key size")
	}
	ret := &blsSignatureScheme{
		priKey: suite.G2().Scalar(),
		pubKey: suite.G2().Point(),
	}
	if err := ret.pubKey.UnmarshalBinary(pubKey); err != nil {
		return nil, err
	}
	if err := ret.priKey.UnmarshalBinary(priKey); err != nil {
		return nil, err
	}
	return ret, nil
}

func (sigscheme *blsSignatureScheme) Version() byte {
	return address.VersionBLS
}

func (sigscheme *blsSignatureScheme) Address() address.Address {
	b, err := sigscheme.pubKey.MarshalBinary()
	if err != nil {
		panic(err)
	}
	return address.FromBLSPubKey(b)
}

func (sigscheme *blsSignatureScheme) Sign(data []byte) Signature {
	sig, err := bdn.Sign(suite, sigscheme.priKey, data)
	if err != nil {
		panic(err)
	}
	pubKeyBin, err := sigscheme.pubKey.MarshalBinary()
	if err != nil {
		panic(err)
	}
	return NewBLSSignature(pubKeyBin, sig)
}

func (sigscheme *blsSignatureScheme) String() string {
	pri, err := sigscheme.priKey.MarshalBinary()
	if err != nil {
		return fmt.Sprintf("BLS sigsheme: %v", err)
	}
	pub, err := sigscheme.pubKey.MarshalBinary()
	if err != nil {
		return fmt.Sprintf("BLS sigsheme: %v", err)
	}
	return base58.Encode(pri) + ", " + base58.Encode(pub)
}

// interface contract (allow the compiler to check if the implementation has all of the required methods).
var _ SignatureScheme = &blsSignatureScheme{}

// ---------------- implements Signature interface

// BLSSignature represents a signature created with the BLS signature scheme.
type BLSSignature [BLSFullSignatureSize]byte

// BLSSignatureFromBytes unmarshals a BLS signature from a sequence of bytes.
func BLSSignatureFromBytes(data []byte) (result *BLSSignature, consumedBytes int, err error) {
	consumedBytes = 0
	err = nil
	if len(data) < BLSFullSignatureSize {
		err = fmt.Errorf("marshaled BLS signature size must be %d", BLSFullSignatureSize)
		return
	}
	if data[0] != address.VersionBLS {
		err = fmt.Errorf("wrong version byte, expected %d", address.VersionBLS)
		return
	}
	result = &BLSSignature{}
	copy(result[:BLSFullSignatureSize], data)
	consumedBytes = BLSFullSignatureSize
	return
}

// NewBLSSignature creates BLS signature from raw public key and signature data
func NewBLSSignature(pubKey, signature []byte) *BLSSignature {
	var ret BLSSignature
	ret[0] = address.VersionBLS
	copy(ret.pubKey(), pubKey)
	copy(ret.signature(), signature)
	return &ret
}

func (sig *BLSSignature) pubKey() []byte {
	return sig[1 : BLSPublicKeySize+1]
}

func (sig *BLSSignature) signature() []byte {
	return sig[1+BLSPublicKeySize:]
}

// IsValid returns true if the signature correctly signs the given data.
func (sig *BLSSignature) IsValid(signedData []byte) bool {
	if sig[0] != address.VersionBLS {
		return false
	}
	// unmarshal public key
	pubKey := suite.G2().Point()
	if err := pubKey.UnmarshalBinary(sig.pubKey()); err != nil {
		return false
	}
	return bdn.Verify(suite, pubKey, signedData, sig.signature()) == nil
}

// Bytes marshals the signature into a sequence of bytes.
func (sig *BLSSignature) Bytes() []byte {
	return sig[:]
}

// Address returns the address that this signature signs.
func (sig *BLSSignature) Address() address.Address {
	return address.FromBLSPubKey(sig.pubKey())
}

func (sig *BLSSignature) String() string {
	return base58.Encode(sig[:])
}

// AggregateBLSSignatures combined multiple Signatures into a single one.
func AggregateBLSSignatures(sigs ...Signature) (Signature, error) {
	if len(sigs) == 0 {
		return nil, fmt.Errorf("must be at least one signature to aggregate")
	}
	if len(sigs) == 1 {
		return sigs[0], nil
	}

	pubKeys := make([]kyber.Point, len(sigs))
	signatures := make([][]byte, len(sigs))

	var err error
	for i, sig := range sigs {
		sigBls, ok := sig.(*BLSSignature)
		if !ok {
			return nil, fmt.Errorf("not a BLS signature")
		}
		pubKeys[i] = suite.G2().Point()
		if err = pubKeys[i].UnmarshalBinary(sigBls.pubKey()); err != nil {
			return nil, err
		}
		signatures[i] = sigBls.signature()
	}
	mask, _ := sign.NewMask(suite, pubKeys, nil)
	for i := range pubKeys {
		_ = mask.SetBit(i, true)
	}
	aggregatedSignature, err := bdn.AggregateSignatures(suite, signatures, mask)
	if err != nil {
		return nil, err
	}
	sigBin, err := aggregatedSignature.MarshalBinary()
	if err != nil {
		return nil, err
	}
	aggregatedPubKey, err := bdn.AggregatePublicKeys(suite, mask)
	if err != nil {
		return nil, err
	}
	pubKeyBin, err := aggregatedPubKey.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return NewBLSSignature(pubKeyBin, sigBin), nil
}

// interface contract (allow the compiler to check if the implementation has all of the required methods).
var _ Signature = &BLSSignature{}
