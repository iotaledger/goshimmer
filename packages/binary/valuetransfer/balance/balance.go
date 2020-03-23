package balance

import (
	"strconv"

	"github.com/iotaledger/hive.go/marshalutil"
)

type Balance struct {
	value int64
	color Color
}

func New(color Color, balance int64) (result *Balance) {
	result = &Balance{
		color: color,
		value: balance,
	}

	return
}

func FromBytes(bytes []byte) (result *Balance, err error, consumedBytes int) {
	result = &Balance{}

	marshalUtil := marshalutil.New(bytes)

	result.value, err = marshalUtil.ReadInt64()
	if err != nil {
		return
	}

	if coinColor, colorErr := marshalUtil.Parse(func(data []byte) (interface{}, error, int) {
		return ColorFromBytes(data)
	}); colorErr != nil {
		return nil, colorErr, marshalUtil.ReadOffset()
	} else {
		result.color = coinColor.(Color)
	}

	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Parse is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func Parse(marshalUtil *marshalutil.MarshalUtil) (*Balance, error) {
	if address, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return FromBytes(data) }); err != nil {
		return nil, err
	} else {
		return address.(*Balance), nil
	}
}

// Value returns the numeric value of the balance.
func (balance *Balance) Value() int64 {
	return balance.value
}

// Color returns the Color of the balance.
func (balance *Balance) Color() Color {
	return balance.color
}

func (balance *Balance) Bytes() []byte {
	marshalUtil := marshalutil.New(Length)

	marshalUtil.WriteInt64(balance.value)
	marshalUtil.WriteBytes(balance.color.Bytes())

	return marshalUtil.Bytes()
}

func (balance *Balance) String() string {
	return strconv.FormatInt(balance.value, 10) + " " + balance.color.String()
}

// Length encodes the length of a marshaled Balance (the length of the color + 8 bytes for the balance).
const Length = 8 + ColorLength
