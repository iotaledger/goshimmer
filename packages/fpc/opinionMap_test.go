package fpc

import (
	"reflect"
	"testing"
)

func TestOpinionMapLen(t *testing.T) {
	type testInput struct {
		opinionMap map[ID][]Opinion
		expected   int
	}
	var tests = []testInput{
		{map[ID][]Opinion{"1": []Opinion{Like, Dislike}}, 1},
		{map[ID][]Opinion{"1": []Opinion{Like, Dislike}, "2": []Opinion{Dislike}}, 2},
		{map[ID][]Opinion{}, 0},
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
		expected map[ID][]Opinion
	}
	var tests = []testInput{
		{map[ID][]Opinion{"1": []Opinion{Like, Dislike}}},
		{map[ID][]Opinion{"1": []Opinion{Like, Dislike}, "2": []Opinion{Dislike}}},
		{map[ID][]Opinion{}},
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
		opinions []Opinion
		ok       bool
	}
	type testInput struct {
		opinionMap map[ID][]Opinion
		key        ID
		expected   expectedResult
	}
	var tests = []testInput{
		{map[ID][]Opinion{"1": []Opinion{Like, Dislike}}, "1", expectedResult{[]Opinion{Like, Dislike}, true}},
		{map[ID][]Opinion{"1": []Opinion{Like, Dislike}}, "2", expectedResult{nil, false}},
		{map[ID][]Opinion{}, "1", expectedResult{nil, false}},
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
		opinionMap map[ID][]Opinion
		key        ID
		value      Opinion
		expected   map[ID][]Opinion
	}
	var tests = []testInput{
		{
			map[ID][]Opinion{"1": []Opinion{Like, Dislike}},
			"1",
			Like,
			map[ID][]Opinion{"1": []Opinion{Like, Dislike, Like}},
		},
		{
			map[ID][]Opinion{"1": []Opinion{Like, Dislike}},
			"2",
			Like,
			map[ID][]Opinion{"1": []Opinion{Like, Dislike}, "2": []Opinion{Like}},
		},
		{
			map[ID][]Opinion{},
			"1",
			Like,
			map[ID][]Opinion{"1": []Opinion{Like}},
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
		opinionMap map[ID][]Opinion
		key        ID
		expected   map[ID][]Opinion
	}
	var tests = []testInput{
		{
			map[ID][]Opinion{"1": []Opinion{Like, Dislike}},
			"1",
			map[ID][]Opinion{},
		},
		{
			map[ID][]Opinion{"1": []Opinion{Like, Dislike}, "2": []Opinion{Like}},
			"2",
			map[ID][]Opinion{"1": []Opinion{Like, Dislike}},
		},
		{
			map[ID][]Opinion{"1": []Opinion{Like, Dislike}},
			"2",
			map[ID][]Opinion{"1": []Opinion{Like, Dislike}},
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
