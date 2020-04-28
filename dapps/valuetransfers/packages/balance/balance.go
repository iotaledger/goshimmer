package balance

import (
	"strconv"

	"github.com/iotaledger/hive.go/marshalutil"
)

// Balance represents a balance in the IOTA ledger. It consists out of a numeric value and a color.
type Balance struct {
	value int64
	color Color
}

// New creates a new Balance with the given details.
func New(color Color, balance int64) (result *Balance) {
	result = &Balance{
		color: color,
		value: balance,
	}

	return
}

// FromBytes unmarshals a Balance from a sequence of bytes.
func FromBytes(bytes []byte) (result *Balance, consumedBytes int, err error) {
	result = &Balance{}

	marshalUtil := marshalutil.New(bytes)

	result.value, err = marshalUtil.ReadInt64()
	if err != nil {
		return
	}

	coinColor, colorErr := marshalUtil.Parse(func(data []byte) (interface{}, int, error) {
		return ColorFromBytes(data)
	})
	if colorErr != nil {
		return nil, marshalUtil.ReadOffset(), colorErr
	}

	result.color = coinColor.(Color)
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

// Value returns the numeric value of the balance.
func (balance *Balance) Value() int64 {
	return balance.value
}

// Color returns the Color of the balance.
func (balance *Balance) Color() Color {
	return balance.color
}

// Bytes marshals the Balance into a sequence of bytes.
func (balance *Balance) Bytes() []byte {
	marshalUtil := marshalutil.New(Length)

	marshalUtil.WriteInt64(balance.value)
	marshalUtil.WriteBytes(balance.color.Bytes())

	return marshalUtil.Bytes()
}

// String creates a human readable string of the Balance.
func (balance *Balance) String() string {
	return strconv.FormatInt(balance.value, 10) + " " + balance.color.String()
}

// Length encodes the length of a marshaled Balance (the length of the color + 8 bytes for the balance).
const Length = 8 + ColorLength
