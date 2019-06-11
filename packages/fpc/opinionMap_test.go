package fpc

import (
	"reflect"
	"testing"
)

func TestOpinionMapLen(t *testing.T) {
	type testInput struct {
		opinionMap map[ID]Opinions
		expected   int
	}
	var tests = []testInput{
		{map[ID]Opinions{"1": Opinions{Like, Dislike}}, 1},
		{map[ID]Opinions{"1": Opinions{Like, Dislike}, "2": Opinions{Dislike}}, 2},
		{map[ID]Opinions{}, 0},
	}

	for _, test := range tests {
		opinionMap := NewOpinionMap()
		opinionMap.internal = test.opinionMap

		result := opinionMap.Len()

		if result != test.expected {
			t.Error("Should return", test.expected, "got", result, "with input", test)
		}
	}
}

func TestOpinionMapGetMap(t *testing.T) {
	type testInput struct {
		expected map[ID]Opinions
	}
	var tests = []testInput{
		{map[ID]Opinions{"1": Opinions{Like, Dislike}}},
		{map[ID]Opinions{"1": Opinions{Like, Dislike}, "2": Opinions{Dislike}}},
		{map[ID]Opinions{}},
	}

	for _, test := range tests {
		opinionMap := NewOpinionMap()
		opinionMap.internal = test.expected

		result := opinionMap.GetMap()

		if !reflect.DeepEqual(result, test.expected) {
			t.Error("Should return", test.expected, "got", result, "with input", test)
		}
	}
}

func TestOpinionMapLoad(t *testing.T) {
	type expectedResult struct {
		opinions Opinions
		ok       bool
	}
	type testInput struct {
		opinionMap map[ID]Opinions
		key        ID
		expected   expectedResult
	}
	var tests = []testInput{
		{map[ID]Opinions{"1": Opinions{Like, Dislike}}, "1", expectedResult{Opinions{Like, Dislike}, true}},
		{map[ID]Opinions{"1": Opinions{Like, Dislike}}, "2", expectedResult{nil, false}},
		{map[ID]Opinions{}, "1", expectedResult{nil, false}},
	}

	for _, test := range tests {
		opinionMap := NewOpinionMap()
		opinionMap.internal = test.opinionMap

		opinions, ok := opinionMap.Load(test.key)

		if !reflect.DeepEqual(opinions, test.expected.opinions) || ok != test.expected.ok {
			t.Error("Should return", test.expected.opinions, test.expected.ok, "got", opinions, ok, "with input", test)
		}
	}
}

func TestOpinionMapStore(t *testing.T) {
	type testInput struct {
		opinionMap map[ID]Opinions
		key        ID
		value      Opinion
		expected   map[ID]Opinions
	}
	var tests = []testInput{
		{
			map[ID]Opinions{"1": Opinions{Like, Dislike}},
			"1",
			Like,
			map[ID]Opinions{"1": Opinions{Like, Dislike, Like}},
		},
		{
			map[ID]Opinions{"1": Opinions{Like, Dislike}},
			"2",
			Like,
			map[ID]Opinions{"1": Opinions{Like, Dislike}, "2": Opinions{Like}},
		},
		{
			map[ID]Opinions{},
			"1",
			Like,
			map[ID]Opinions{"1": Opinions{Like}},
		},
	}

	for _, test := range tests {
		opinionMap := NewOpinionMap()
		opinionMap.internal = test.opinionMap

		opinionMap.Store(test.key, test.value)

		if !reflect.DeepEqual(opinionMap.internal, test.expected) {
			t.Error("Should return", test.expected, "got", opinionMap.internal, "with input", test)
		}
	}
}

func TestOpinionMapDelete(t *testing.T) {
	type testInput struct {
		opinionMap map[ID]Opinions
		key        ID
		expected   map[ID]Opinions
	}
	var tests = []testInput{
		{
			map[ID]Opinions{"1": Opinions{Like, Dislike}},
			"1",
			map[ID]Opinions{},
		},
		{
			map[ID]Opinions{"1": Opinions{Like, Dislike}, "2": Opinions{Like}},
			"2",
			map[ID]Opinions{"1": Opinions{Like, Dislike}},
		},
		{
			map[ID]Opinions{"1": Opinions{Like, Dislike}},
			"2",
			map[ID]Opinions{"1": Opinions{Like, Dislike}},
		},
	}

	for _, test := range tests {
		opinionMap := NewOpinionMap()
		opinionMap.internal = test.opinionMap

		opinionMap.Delete(test.key)

		if !reflect.DeepEqual(opinionMap.internal, test.expected) {
			t.Error("Should return", test.expected, "got", opinionMap.internal, "with input", test)
		}
	}
}
