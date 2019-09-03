package salt

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	pb "github.com/wollac/autopeering/salt/proto"
)

// SaltByteSize specifies the number of bytes used for the salt.
const SaltByteSize = 20

// Salt encapsulates high level functions around salt management.
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

// ToProto encodes the Salt into a proto buffer Salt message
func (s *Salt) ToProto() *pb.Salt {
	return &pb.Salt{
		Bytes:   s.Bytes,
		ExpTime: uint64(s.ExpirationTime.Unix()),
	}
}

// FromProto decodes a given proto buffer Salt message (in) and returns the corresponding Salt.
func FromProto(in *pb.Salt) (*Salt, error) {
	if l := len(in.GetBytes()); l != SaltByteSize {
		return nil, fmt.Errorf("invalid salt length: %d, need %d", l, SaltByteSize)
	}
	out := &Salt{
		Bytes:          in.GetBytes(),
		ExpirationTime: time.Unix(int64(in.GetExpTime()), 0),
	}
	return out, nil
}

// Marshal serializes a given salt (s) into a slice of bytes (data)
func (s *Salt) Marshal() ([]byte, error) {
	return proto.Marshal(s.ToProto())
}

// Unmarshal deserializes a given slice of bytes (data) into a Salt.
func Unmarshal(data []byte) (*Salt, error) {
	s := &pb.Salt{}
	if err := proto.Unmarshal(data, s); err != nil {
		return nil, err
	}
	return FromProto(s)
}
