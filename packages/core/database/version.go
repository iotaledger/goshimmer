package database

import "github.com/pkg/errors"

type Version byte

func (v *Version) Bytes() (bytes []byte, err error) {
	return []byte{byte(*v)}, nil
}

func (v *Version) FromBytes(bytes []byte) (consumedBytes int, err error) {
	if len(bytes) == 0 {
		return 0, errors.Errorf("not enough bytes")
	}

	*v = Version(bytes[0])
	return 1, nil
}
