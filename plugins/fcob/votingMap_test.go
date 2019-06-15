package fcob

import (
	"reflect"
	"testing"

	"github.com/iotaledger/goshimmer/packages/ternary"
)

func TestVotingMapLen(t *testing.T) {
	type testInput struct {
		votingMap map[ternary.Trinary]bool
		expected  int
	}
	var tests = []testInput{
		{map[ternary.Trinary]bool{"1": true}, 1},
		{map[ternary.Trinary]bool{"1": true, "2": true}, 2},
		{map[ternary.Trinary]bool{}, 0},
	}

	for _, test := range tests {
		votingMap := NewVotingMap()
		votingMap.internal = test.votingMap

		result := votingMap.Len()

		if result != test.expected {
			t.Error("Should return", test.expected, "got", result, "with input", test)
		}
	}
}

func TestVotingMapGetMap(t *testing.T) {
	type testInput struct {
		expected map[ternary.Trinary]bool
	}
	var tests = []testInput{
		{map[ternary.Trinary]bool{"1": true}},
		{map[ternary.Trinary]bool{"1": true, "2": true}},
		{map[ternary.Trinary]bool{}},
	}

	for _, test := range tests {
		votingMap := NewVotingMap()
		votingMap.internal = test.expected

		result := votingMap.GetMap()

		if !reflect.DeepEqual(result, test.expected) {
			t.Error("Should return", test.expected, "got", result, "with input", test)
		}
	}
}

func TestVotingMapLoad(t *testing.T) {

	type testInput struct {
		votingMap map[ternary.Trinary]bool
		key       ternary.Trinary
		expected  bool
	}
	var tests = []testInput{
		{map[ternary.Trinary]bool{"1": true}, "1", true},
		{map[ternary.Trinary]bool{"1": true}, "2", false},
		{map[ternary.Trinary]bool{}, "1", false},
	}

	for _, test := range tests {
		votingMap := NewVotingMap()
		votingMap.internal = test.votingMap

		ok := votingMap.Load(test.key)

		if !reflect.DeepEqual(ok, test.expected) {
			t.Error("Should return", test.expected, "got", ok, "with input", test)
		}
	}
}

func TestVotingMapStore(t *testing.T) {
	type testInput struct {
		votingMap map[ternary.Trinary]bool
		keys      []ternary.Trinary
		expected  map[ternary.Trinary]bool
	}
	var tests = []testInput{
		{
			map[ternary.Trinary]bool{"1": true},
			[]ternary.Trinary{"2"},
			map[ternary.Trinary]bool{"1": true, "2": true},
		},
		{
			map[ternary.Trinary]bool{"1": true},
			[]ternary.Trinary{"2", "3"},
			map[ternary.Trinary]bool{"1": true, "2": true, "3": true},
		},
		{
			map[ternary.Trinary]bool{"1": true},
			[]ternary.Trinary{"1"},
			map[ternary.Trinary]bool{"1": true},
		},
		{
			map[ternary.Trinary]bool{},
			[]ternary.Trinary{"1"},
			map[ternary.Trinary]bool{"1": true},
		},
	}

	for _, test := range tests {
		votingMap := NewVotingMap()
		votingMap.internal = test.votingMap

		votingMap.Store(test.keys...)

		if !reflect.DeepEqual(votingMap.internal, test.expected) {
			t.Error("Should return", test.expected, "got", votingMap.internal, "with input", test)
		}
	}
}

func TestVotingMapDelete(t *testing.T) {
	type testInput struct {
		votingMap map[ternary.Trinary]bool
		key       ternary.Trinary
		expected  map[ternary.Trinary]bool
	}
	var tests = []testInput{
		{
			map[ternary.Trinary]bool{"1": true},
			"1",
			map[ternary.Trinary]bool{},
		},
		{
			map[ternary.Trinary]bool{"1": true},
			"2",
			map[ternary.Trinary]bool{"1": true},
		},
		{
			map[ternary.Trinary]bool{"1": true, "2": true},
			"1",
			map[ternary.Trinary]bool{"2": true},
		},
		{
			map[ternary.Trinary]bool{},
			"1",
			map[ternary.Trinary]bool{},
		},
	}

	for _, test := range tests {
		votingMap := NewVotingMap()
		votingMap.internal = test.votingMap

		votingMap.Delete(test.key)

		if !reflect.DeepEqual(votingMap.internal, test.expected) {
			t.Error("Should return", test.expected, "got", votingMap.internal, "with input", test)
		}
	}
}
