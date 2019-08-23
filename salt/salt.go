package salt

import (
	"crypto/rand"
	"time"

	"github.com/golang/protobuf/proto"
	pb "github.com/wollac/autopeering/salt/proto"
)

const SaltByteSize = 20

type Salt struct {
	Bytes          []byte    // value of the salt
	ExpirationTime time.Time // expiration time of the salt
}

// NewSalt generates a new salt given a lifetime duration
func NewSalt(lifetime time.Duration) (salt *Salt, err error) {
	salt = &Salt{
		Bytes:          make([]byte, SaltByteSize),
		ExpirationTime: time.Now().Add(lifetime),
	}

	if _, err = rand.Read(salt.Bytes); err != nil {
		return nil, err
	}

	return salt, err
}

// Expired returns true if the given salt expired
func (s *Salt) Expired() bool {
	return time.Now().After(s.ExpirationTime)
}

// Encode encodes a given Salt (s) into a proto buffer Salt message
func Encode(s *Salt) (result *pb.Salt, err error) {
	result = &pb.Salt{}
	result.ExpTime, err = s.ExpirationTime.MarshalBinary()
	if err != nil {
		return nil, err
	}
	result.Value = s.Bytes
	return
}

// Decode decodes a given proto buffer Salt message (in) into a Salt (out)
// out MUST NOT be nil
func Decode(in *pb.Salt, out *Salt) (err error) {
	err = out.ExpirationTime.UnmarshalBinary(in.GetExpTime())
	if err != nil {
		return err
	}
	out.Bytes = in.GetValue()
	return
}

// Marshal serializes a given salt (s) into a slice of bytes (data)
func Marshal(s *Salt) (data []byte, err error) {
	pb, err := Encode(s)
	if err != nil {
		return nil, err
	}
	return proto.Marshal(pb)
}

// Unmarshal deserializes a given slice of bytes (data) into a Salt (out)
// out MUST NOT be nil
func Unmarshal(data []byte, out *Salt) (err error) {
	s := &pb.Salt{}
	err = proto.Unmarshal(data, s)
	if err != nil {
		return err
	}
	return Decode(s, out)
}
