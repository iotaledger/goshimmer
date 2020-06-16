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
				NodesView: map[string]voteContext{
					"one": {Status: liked},
					"two": {Status: disliked},
				},
			},
			want: true,
		},
		{
			conflict: conflict{
				NodesView: map[string]voteContext{
					"one": {Status: liked},
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
				NodesView: map[string]voteContext{
					"one": {Status: liked},
					"two": {Status: disliked},
				},
			},
			want: 1,
		},
		{
			conflict: conflict{
				NodesView: map[string]voteContext{
					"one": {Status: liked},
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
