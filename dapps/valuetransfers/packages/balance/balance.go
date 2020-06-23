package balance

import (
	"strconv"

	"github.com/iotaledger/hive.go/marshalutil"
)

// Balance represents a balance in the IOTA ledger. It consists out of a numeric value and a color.
type Balance struct {
	// The numeric value of the balance.
	Value int64 `json:"value"`
	// The color of the balance.
	Color Color `json:"color"`
}

// New creates a new Balance with the given details.
func New(color Color, balance int64) (result *Balance) {
	result = &Balance{
		Color: color,
		Value: balance,
	}

	return
}

// FromBytes unmarshals a Balance from a sequence of bytes.
func FromBytes(bytes []byte) (result *Balance, consumedBytes int, err error) {
	result = &Balance{}

	marshalUtil := marshalutil.New(bytes)

	result.Value, err = marshalUtil.ReadInt64()
	if err != nil {
		return
	}

	coinColor, colorErr := marshalUtil.Parse(func(data []byte) (interface{}, int, error) {
		return ColorFromBytes(data)
	})
	if colorErr != nil {
		return nil, marshalUtil.ReadOffset(), colorErr
	}

	result.Color = coinColor.(Color)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Parse is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func Parse(marshalUtil *marshalutil.MarshalUtil) (*Balance, error) {
	address, err := marshalUtil.Parse(func(data []byte) (interface{}, int, error) { return FromBytes(data) })
	if err != nil {
		return nil, err
	}

	return address.(*Balance), nil
}

// Bytes marshals the Balance into a sequence of bytes.
func (balance *Balance) Bytes() []byte {
	marshalUtil := marshalutil.New(Length)

	marshalUtil.WriteInt64(balance.Value)
	marshalUtil.WriteBytes(balance.Color.Bytes())

	return marshalUtil.Bytes()
}

// String creates a human readable string of the Balance.
func (balance *Balance) String() string {
	return strconv.FormatInt(balance.Value, 10) + " " + balance.Color.String()
}

// Length encodes the length of a marshaled Balance (the length of the color + 8 bytes for the balance).
const Length = 8 + ColorLength
