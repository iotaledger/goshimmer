package signaturescheme

import (
	"bytes"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing/bn256"
	"go.dedis.ch/kyber/v3/sign/bdn"
	"go.dedis.ch/kyber/v3/util/random"
)

// BLS_BDN implements BLS signature scheme which is robust agains rogue public key attacks
// it uses go.dedis/kyber library
// more info https://github.com/dedis/kyber/blob/master/sign/bdn/bdn.go
// usually BLS signatures are used as threshold signatures.
// This package don't implement any threshold signature related primitives.
// it only implements what is needed for the node to check validity of the BLS signatures against addresses
// and also minimum signing required for testing

var suite = bn256.NewSuite()

// blsBdnSignatureScheme defines an interface for the BLS signatures.
// here it is unmarshaled because only used for testing

type blsBdnSignatureScheme struct {
	privKey kyber.Scalar
	pubKey  kyber.Point
}

// RandBLS_BDN creates an RANDOM instance of a signature scheme, that is used to sign the corresponding address.
// mostly intended for testing.
func RandBLS_BDN() SignatureScheme {
	ret := &blsBdnSignatureScheme{}
	ret.privKey, ret.pubKey = bdn.NewKeyPair(suite, random.New()) // only for testing: deterministic!
	return ret
}

func (sigscheme *blsBdnSignatureScheme) Version() byte {
	suite.ScalarLen()
	return address.VERSION_BLS_BDN
}

func (sigscheme *blsBdnSignatureScheme) Address() address.Address {
	b, err := sigscheme.pubKey.MarshalBinary()
	if err != nil {
		panic(err)
	}
	return address.FromBLSPubKey(b)
}

func (sigscheme *blsBdnSignatureScheme) Sign(data []byte) Signature {
	sig, err := bdn.Sign(suite, sigscheme.privKey, data)
	if err != nil {
		panic(err)
	}
	pubKeyBin, err := sigscheme.pubKey.MarshalBinary()
	if err != nil {
		panic(err)
	}
	return &blsBdnSignature{
		pubKey:    pubKeyBin,
		signature: sig,
	}
}

// interface contract (allow the compiler to check if the implementation has all of the required methods).
var _ SignatureScheme = &blsBdnSignatureScheme{}

// BLS-BDN signature, in binary marshaled form

type blsBdnSignature struct {
	pubKey    []byte
	signature []byte
}

func (sig *blsBdnSignature) IsValid(signedData []byte) bool {
	// unmarshal public key
	pubKey := suite.G2().Point()
	if err := pubKey.UnmarshalBinary(sig.pubKey); err != nil {
		return false
	}
	return bdn.Verify(suite, pubKey, signedData, sig.signature) == nil
}

func (sig *blsBdnSignature) Bytes() []byte {
	b := make([]byte, 1+len(sig.pubKey)+len(sig.signature))
	buf := bytes.NewBuffer(b)
	buf.WriteByte(address.VERSION_BLS_BDN)
	buf.Write(sig.pubKey)
	buf.Write(sig.signature)
	return b
}

func (sig *blsBdnSignature) Address() address.Address {
	return address.FromBLSPubKey(sig.pubKey)
}

// interface contract (allow the compiler to check if the implementation has all of the required methods).
var _ Signature = &blsBdnSignature{}
