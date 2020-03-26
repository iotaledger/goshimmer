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

	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
)

// BLS implements BLS signature scheme which is robust against rogue public key attacks, or BDN
// it uses go.dedis/kyber library
// more info https://github.com/dedis/kyber/blob/master/sign/bdn/bdn.go
// usually BLS signatures are used as threshold signatures.
// This package doesn't implement any threshold signature related primitives.
// it only contains what is needed for the node to check validity of the BLS signatures against addresses
// and also minimum signing required for testing
var suite = bn256.NewSuite()

const (
	BLS_SIGNATURE_SIZE      = 64
	BLS_PUBLIC_KEY_SIZE     = 128
	BLS_PRIVATE_KEY_SIZE    = 32
	BLS_FULL_SIGNATURE_SIZE = 1 + BLS_PUBLIC_KEY_SIZE + BLS_SIGNATURE_SIZE
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
// mostly intended for testing.
// only for testing: each time same sequence!
func RandBLS() SignatureScheme {
	ret := &blsSignatureScheme{}
	ret.priKey, ret.pubKey = bdn.NewKeyPair(suite, rnd)
	return ret
}

// BLS creates an instance of BLS signature scheme
// from given private and public keys in marshaled binary form
func BLS(priKey, pubKey []byte) (SignatureScheme, error) {
	if len(priKey) != BLS_PRIVATE_KEY_SIZE || len(pubKey) != BLS_PUBLIC_KEY_SIZE {
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
	return address.VERSION_BLS
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
	return newBLSSignature(pubKeyBin, sig)
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

type blsSignature [BLS_FULL_SIGNATURE_SIZE]byte

func BLSSignatureFromBytes(data []byte) (result *blsSignature, err error, consumedBytes int) {
	consumedBytes = 0
	err = nil
	if len(data) < BLS_FULL_SIGNATURE_SIZE {
		err = fmt.Errorf("marshaled BLS signature size must be %d", BLS_FULL_SIGNATURE_SIZE)
		return
	}
	if data[0] != address.VERSION_BLS {
		err = fmt.Errorf("wrong version byte, expected %d", address.VERSION_BLS)
		return
	}
	result = &blsSignature{}
	copy(result[:BLS_FULL_SIGNATURE_SIZE], data)
	consumedBytes = BLS_FULL_SIGNATURE_SIZE
	return
}

func newBLSSignature(pubKey, signature []byte) *blsSignature {
	var ret blsSignature
	ret[0] = address.VERSION_BLS
	copy(ret.pubKey(), pubKey)
	copy(ret.signature(), signature)
	return &ret
}

func (sig *blsSignature) pubKey() []byte {
	return sig[1 : BLS_PUBLIC_KEY_SIZE+1]
}

func (sig *blsSignature) signature() []byte {
	return sig[1+BLS_PUBLIC_KEY_SIZE:]
}

func (sig *blsSignature) IsValid(signedData []byte) bool {
	if sig[0] != address.VERSION_BLS {
		return false
	}
	// unmarshal public key
	pubKey := suite.G2().Point()
	if err := pubKey.UnmarshalBinary(sig.pubKey()); err != nil {
		return false
	}
	return bdn.Verify(suite, pubKey, signedData, sig.signature()) == nil
}

func (sig *blsSignature) Bytes() []byte {
	return sig[:]
}

func (sig *blsSignature) Address() address.Address {
	return address.FromBLSPubKey(sig.pubKey())
}

func (sig *blsSignature) String() string {
	return base58.Encode(sig[:])
}

func AggregateBLSSignatureSchemes(sigSchemes ...SignatureScheme) (SignatureScheme, error) {
	priKeys := make([]kyber.Scalar, len(sigSchemes))
	pubKeys := make([]kyber.Point, len(sigSchemes))
	for i, s := range sigSchemes {
		ss, ok := s.(*blsSignatureScheme)
		if !ok {
			return nil, fmt.Errorf("not a BLS signature scheme")
		}
		priKeys[i] = ss.priKey
		pubKeys[i] = ss.pubKey
	}
	aggregatedPriKey := suite.G2().Scalar().Zero()
	// sum up all private keys
	for i := range priKeys {
		aggregatedPriKey = aggregatedPriKey.Add(aggregatedPriKey, priKeys[i])
	}
	mask, _ := sign.NewMask(suite, pubKeys, nil)
	for i := range pubKeys {
		_ = mask.SetBit(i, true)
	}
	aggregatedPubKey, err := bdn.AggregatePublicKeys(suite, mask)
	if err != nil {
		return nil, err
	}
	return &blsSignatureScheme{
		priKey: aggregatedPriKey,
		pubKey: aggregatedPubKey,
	}, nil
}

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
		sigBls, ok := sig.(*blsSignature)
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
	return newBLSSignature(pubKeyBin, sigBin), nil
}

// interface contract (allow the compiler to check if the implementation has all of the required methods).
var _ Signature = &blsSignature{}
