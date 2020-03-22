package balance

import (
	"strconv"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
)

type Balance struct {
	color   Color
	balance int64
}

func New(color Color, balance int64) (result *Balance) {
	result = &Balance{
		color:   color,
		balance: balance,
	}

	return
}

func FromBytes(bytes []byte) (result *Balance, err error, consumedBytes int) {
	result = &Balance{}

	marshalUtil := marshalutil.New(bytes)

	if coinColor, colorErr := marshalUtil.Parse(func(data []byte) (interface{}, error, int) {
		return ColorFromBytes(data)
	}); colorErr != nil {
		return nil, colorErr, marshalUtil.ReadOffset()
	} else {
		result.color = coinColor.(Color)
	}

	result.balance, err = marshalUtil.ReadInt64()
	if err != nil {
		return
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

func (balance *Balance) Bytes() []byte {
	marshalUtil := marshalutil.New(Length)

	marshalUtil.WriteBytes(balance.color.Bytes())
	marshalUtil.WriteInt64(balance.balance)

	return marshalUtil.Bytes()
}

func (balance *Balance) String() string {
	return strconv.FormatInt(balance.balance, 10) + " " + balance.color.String()
}

// Length encodes the length of a marshaled Balance (the length of the color + 8 bytes for the balance).
const Length = ColorLength + 8
