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

// ToProto encodes a given Salt (s) into a proto buffer Salt message
func ToProto(s *Salt) (result *pb.Salt, err error) {
	result = &pb.Salt{}
	result.ExpTime = uint64(s.ExpirationTime.Unix())
	result.Bytes = s.Bytes
	return
}

// FromProto decodes a given proto buffer Salt message (in) into a Salt (out)
// out MUST NOT be nil
func FromProto(in *pb.Salt, out *Salt) (err error) {
	if out == nil {
		return ErrNilInput
	}
	out.ExpirationTime = time.Unix(int64(in.GetExpTime()), 0)
	out.Bytes = in.GetBytes()
	return
}

// Marshal serializes a given salt (s) into a slice of bytes (data)
func Marshal(s *Salt) (data []byte, err error) {
	pb, err := ToProto(s)
	if err != nil {
		return nil, ErrMarshal
	}
	return proto.Marshal(pb)
}

// Unmarshal deserializes a given slice of bytes (data) into a Salt (out)
// out MUST NOT be nil
func Unmarshal(data []byte, out *Salt) (err error) {
	if out == nil {
		return ErrNilInput
	}
	s := &pb.Salt{}
	err = proto.Unmarshal(data, s)
	if err != nil {
		return ErrUnmarshal
	}
	return FromProto(s, out)
}
