package dashboard

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIsFinalized checks that for a given conflict, its method isFinalized works ok.
func TestIsFinalized(t *testing.T) {
	tests := []struct {
		conflict
		want bool
	}{
		{
			conflict: conflict{
				nodesView: map[string]voteContext{
					"one": {status: liked},
					"two": {status: disliked},
				},
			},
			want: true,
		},
		{
			conflict: conflict{
				nodesView: map[string]voteContext{
					"one": {status: liked},
					"two": {},
				},
			},
			want: false,
		},
		{
			conflict: conflict{},
			want:     false,
		},
	}

	for _, conflictTest := range tests {
		require.Equal(t, conflictTest.want, conflictTest.isFinalized())
	}

}

// TestFinalizationStatus checks that for a given conflict, its method finalizationStatus works ok.
func TestFinalizationStatus(t *testing.T) {
	tests := []struct {
		conflict
		want float64
	}{
		{
			conflict: conflict{
				nodesView: map[string]voteContext{
					"one": {status: liked},
					"two": {status: disliked},
				},
			},
			want: 1,
		},
		{
			conflict: conflict{
				nodesView: map[string]voteContext{
					"one": {status: liked},
					"two": {},
				},
			},
			want: 0.5,
		},
		{
			conflict: conflict{},
			want:     0,
		},
	}

	for _, conflictTest := range tests {
		require.Equal(t, conflictTest.want, conflictTest.finalizationStatus())
	}

}
