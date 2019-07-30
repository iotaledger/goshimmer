package iac

import (
	"math"

	olc "github.com/google/open-location-code/go"
	"github.com/iotaledger/iota.go/trinary"
)

type Area struct {
	olc.CodeArea
	IACCode trinary.Trytes
	OLCCode string
}

func (area *Area) Distance(other *Area) float64 {
	lat1, lng1 := area.Center()
	lat2, lng2 := other.Center()

	return distance(lat1, lng1, lat2, lng2)
}

func distance(lat1, lon1, lat2, lon2 float64) float64 {
	la1 := lat1 * math.Pi / 180
	lo1 := lon1 * math.Pi / 180
	la2 := lat2 * math.Pi / 180
	lo2 := lon2 * math.Pi / 180

	return 2 * EARTH_RADIUS_IN_METERS * math.Asin(math.Sqrt(hsin(la2-la1)+math.Cos(la1)*math.Cos(la2)*hsin(lo2-lo1)))
}

func hsin(theta float64) float64 {
	return math.Pow(math.Sin(theta/2), 2)
}

const (
	EARTH_RADIUS_IN_METERS = 6371000
)
