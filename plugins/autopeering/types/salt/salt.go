package salt

import (
	"crypto/rand"
	"time"

	"github.com/pkg/errors"
)

type Salt struct {
	Bytes          []byte
	ExpirationTime time.Time
}

func New(lifetime time.Duration) *Salt {
	salt := &Salt{
		Bytes:          make([]byte, SALT_BYTES_LEN),
		ExpirationTime: time.Now().Add(lifetime),
	}

	if _, err := rand.Read(salt.Bytes); err != nil {
		panic(err)
	}

	return salt
}

func Unmarshal(data []byte) (*Salt, error) {
	if len(data) != MARSHALLED_TOTAL_SIZE {
		return nil, errors.New("salt: bad data length")
	}

	salt := &Salt{
		Bytes: make([]byte, SALT_BYTES_LEN),
	}
	copy(salt.Bytes, data[MARSHALLED_BYTES_START:MARSHALLED_BYTES_END])

	if err := salt.ExpirationTime.UnmarshalBinary(data[MARSHALLED_TIME_START:MARSHALLED_TIME_END]); err != nil {
		return nil, err
	}

	return salt, nil
}

func (s *Salt) Marshal() []byte {
	data := make([]byte, MARSHALLED_TOTAL_SIZE)

	copy(data[MARSHALLED_BYTES_START:MARSHALLED_BYTES_END], s.Bytes)

	if bytes, err := s.ExpirationTime.MarshalBinary(); err != nil {
		panic(err)
	} else {
		copy(data[MARSHALLED_TIME_START:MARSHALLED_TIME_END], bytes)
	}

	return data
}
