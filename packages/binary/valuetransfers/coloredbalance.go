package valuetransfers

import (
	"strconv"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
)

type ColoredBalance struct {
	color   Color
	balance int64
}

func NewColoredBalance(color Color, balance int64) (result *ColoredBalance) {
	result = &ColoredBalance{
		color:   color,
		balance: balance,
	}

	return
}

func ColoredBalanceFromBytes(bytes []byte) (result *ColoredBalance, err error, consumedBytes int) {
	result = &ColoredBalance{}

	marshalUtil := marshalutil.New(bytes)

	if color, colorErr := marshalUtil.Parse(func(data []byte) (interface{}, error, int) {
		return ColorFromBytes(data)
	}); colorErr != nil {
		return nil, colorErr, marshalUtil.ReadOffset()
	} else {
		result.color = color.(Color)
	}

	result.balance, err = marshalUtil.ReadInt64()
	if err != nil {
		return
	}

	consumedBytes = marshalUtil.ReadOffset()

	return
}

func (balance *ColoredBalance) ToBytes() []byte {
	marshalUtil := marshalutil.New(ColorLength + marshalutil.UINT32_SIZE)

	marshalUtil.WriteBytes(balance.color.Bytes())
	marshalUtil.WriteInt64(balance.balance)

	return marshalUtil.Bytes()
}

func (balance *ColoredBalance) String() string {
	return strconv.FormatInt(balance.balance, 10) + " " + balance.color.String()
}
