package fpc

import (
	"reflect"
	"testing"
)

func TestNewContext(t *testing.T) {
	param := Parameters{
		a:           0.75,
		b:           0.85,
		beta:        0.33,
		k:           10,
		m:           2,
		l:           2,
		MaxDuration: 100,
	}
	type testInput struct {
		p        *Parameters
		expected Parameters
	}
	var tests = []testInput{
		{nil, param},
		{&param, param},
	}

	for _, test := range tests {
		result := newContext(test.p)

		if !reflect.DeepEqual(*result.parameters, test.expected) {
			t.Error("Should return", test.expected, "got", result.parameters, "with input", test)
		}
	}
}

func TestTxQueueLen(t *testing.T) {
	type testInput struct {
		queue    []TxOpinion
		expected int
	}
	var tests = []testInput{
		{[]TxOpinion{}, 0},
		{[]TxOpinion{{"1", Like}}, 1},
		{[]TxOpinion{{"1", Like}, {"2", Dislike}}, 2},
	}
	for _, test := range tests {
		queue := newTxQueue()
		queue.internal = test.queue

		result := queue.Len()

		if result != test.expected {
			t.Error("Should return", test.expected, "got", result, "with input", test)
		}
	}
}

func TestTxQueuePush(t *testing.T) {
	type testInput struct {
		queue    []TxOpinion
		newTxs   []TxOpinion
		expected []TxOpinion
	}
	var tests = []testInput{
		{[]TxOpinion{}, []TxOpinion{}, []TxOpinion{}},
		{[]TxOpinion{}, nil, []TxOpinion{}},
		{[]TxOpinion{{"1", Like}}, []TxOpinion{{"2", Like}}, []TxOpinion{{"1", Like}, {"2", Like}}},
		{[]TxOpinion{{"1", Like}}, []TxOpinion{}, []TxOpinion{{"1", Like}}},
	}
	for _, test := range tests {
		queue := newTxQueue()
		queue.internal = test.queue

		queue.Push(test.newTxs...)

		if !reflect.DeepEqual(queue.internal, test.expected) {
			t.Error("Should return", test.expected, "got", queue.internal, "with input", test)
		}
	}
}

func TestTxQueuePop(t *testing.T) {
	type testInput struct {
		queue    []TxOpinion
		n        []uint
		expected []TxOpinion
	}
	var tests = []testInput{
		{[]TxOpinion{}, []uint{}, nil},
		{[]TxOpinion{{"1", Like}}, []uint{1}, []TxOpinion{}},
		{[]TxOpinion{{"1", Like}, {"2", Like}}, []uint{1}, []TxOpinion{{"2", Like}}},
		{[]TxOpinion{{"1", Like}, {"2", Like}}, []uint{}, nil},
	}
	for _, test := range tests {
		queue := newTxQueue()
		queue.internal = test.queue

		queue.Pop(test.n...)

		if !reflect.DeepEqual(queue.internal, test.expected) {
			t.Error("Should return", test.expected, "got", queue.internal, "with input", test)
		}
	}
}

func TestGetActiveTxs(t *testing.T) {
	type testInput struct {
		activeTxs etaMap
		expected  []ID
	}
	var tests = []testInput{
		{etaMap{"1": &etaResult{1, 1}}, []ID{"1"}},
		{etaMap{}, []ID{}},
	}
	for _, test := range tests {
		c := newContext()
		c.activeTxs = test.activeTxs

		result := c.getActiveTxs()

		if !reflect.DeepEqual(result, test.expected) {
			t.Error("Should return", test.expected, "got", result, "with input", test)
		}
	}
}
