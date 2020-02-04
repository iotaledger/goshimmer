package salt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSalt(t *testing.T) {
	type testCase struct {
		input time.Duration
		want  error
	}

	tests := []testCase{
		{input: 0, want: nil},
		{input: 10, want: nil},
		{input: -1, want: nil},
	}

	for _, test := range tests {
		_, err := NewSalt(test.input)
		assert.Equal(t, test.want, err, test)
	}
}

func TestSaltExpired(t *testing.T) {
	type testCase struct {
		input time.Duration
		want  bool
	}

	tests := []testCase{
		{input: 0, want: true},
		{input: time.Second * 10, want: false},
		{input: -1, want: true},
	}

	for _, test := range tests {
		salt, _ := NewSalt(test.input)
		got := salt.Expired()
		assert.Equal(t, test.want, got, test)
	}
}

func TestMarshalUnmarshal(t *testing.T) {
	type testCase struct {
		input time.Duration
	}

	tests := []testCase{
		{input: 0},
		{input: time.Second * 10},
		{input: -1},
	}

	for _, test := range tests {
		salt, _ := NewSalt(test.input)

		data, err := salt.Marshal()
		require.Equal(t, nil, err, "NoErrorCheck")

		got, err := Unmarshal(data)
		require.Equal(t, nil, err, "NoErrorCheck")

		assert.Equal(t, salt.GetBytes(), got.GetBytes(), "Salt")
		assert.Equal(t, salt.GetExpiration().Unix(), got.GetExpiration().Unix(), "SameSaltExpirationTime")

	}

}
