package fcob

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/fpc"
)

func TestBoolToOpionion(t *testing.T) {
	type testInput struct {
		input    bool
		expected fpc.Opinion
	}
	var tests = []testInput{
		{true, fpc.Like},
		{false, fpc.Dislike},
	}

	for _, test := range tests {
		result := boolToOpinion(test.input)

		if result != test.expected {
			t.Error("Should return", test.expected, "got", result, "with input", test)
		}
	}
}

func TestOpionionToBool(t *testing.T) {
	type testInput struct {
		input    Opinion
		expected bool
	}
	var tests = []testInput{
		{Opinion{fpc.Like, true}, true},
		{Opinion{fpc.Like, false}, true},
		{Opinion{fpc.Dislike, true}, false},
		{Opinion{fpc.Dislike, false}, false},
	}

	for _, test := range tests {
		result := opinionToBool(test.input)

		if result != test.expected {
			t.Error("Should return", test.expected, "got", result, "with input", test)
		}
	}
}

func TestOpionionVoted(t *testing.T) {
	type testInput struct {
		input    Opinion
		expected bool
	}
	var tests = []testInput{
		{Opinion{fpc.Like, true}, true},
		{Opinion{fpc.Like, false}, false},
		{Opinion{fpc.Dislike, true}, true},
		{Opinion{fpc.Dislike, false}, false},
	}

	for _, test := range tests {
		result := test.input.voted()

		if result != test.expected {
			t.Error("Should return", test.expected, "got", result, "with input", test)
		}
	}
}

func TestOpionionLiked(t *testing.T) {
	type testInput struct {
		input    Opinion
		expected bool
	}
	var tests = []testInput{
		{Opinion{fpc.Like, true}, true},
		{Opinion{fpc.Like, false}, true},
		{Opinion{fpc.Dislike, true}, false},
		{Opinion{fpc.Dislike, false}, false},
	}

	for _, test := range tests {
		result := test.input.liked()

		if result != test.expected {
			t.Error("Should return", test.expected, "got", result, "with input", test)
		}
	}
}
