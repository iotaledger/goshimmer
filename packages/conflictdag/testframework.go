package conflictdag

import (
	"github.com/iotaledger/hive.go/crypto"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/mr-tron/base58"
)

type MockedConflictID [32]byte

func (m MockedConflictID) Unmarshal(marshalUtil *marshalutil.MarshalUtil) (conflict MockedConflictID, err error) {
	readBytes, err := marshalUtil.ReadBytes(32)
	if err != nil {
		return conflict, err
	}
	copy(conflict[:], readBytes)

	return conflict, nil
}

func (m MockedConflictID) Bytes() (serialized []byte) {
	return m[:]
}

func (m MockedConflictID) Base58() (base58Encoded string) {
	return base58.Encode(m.Bytes())
}

func (m MockedConflictID) String() (humanReadable string) {
	return "MockedConflictID(" + m.Base58() + ")"
}

func (m *MockedConflictID) FromRandomness() (err error) {
	_, err = crypto.Randomness.Read((*m)[:])

	return err
}

type MockedConflictSetID [32]byte

func (m MockedConflictSetID) Unmarshal(marshalUtil *marshalutil.MarshalUtil) (conflict MockedConflictSetID, err error) {
	readBytes, err := marshalUtil.ReadBytes(32)
	if err != nil {
		return conflict, err
	}
	copy(conflict[:], readBytes)

	return conflict, nil
}

func (m MockedConflictSetID) Bytes() (serialized []byte) {
	return m[:]
}

func (m MockedConflictSetID) Base58() (base58Encoded string) {
	return base58.Encode(m.Bytes())
}

func (m MockedConflictSetID) String() (humanReadable string) {
	return "MockedConflictSetID(" + m.Base58() + ")"
}

func (m *MockedConflictSetID) FromRandomness() (err error) {
	_, err = crypto.Randomness.Read((*m)[:])

	return err
}
