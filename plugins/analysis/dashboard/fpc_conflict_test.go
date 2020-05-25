package dashboard

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIsFinalized checks that for a given conflict, its method isFinalized works ok.
func TestIsFinalized(t *testing.T) {
	tests := []struct {
		Conflict
		want bool
	}{
		{
			Conflict: Conflict{
				NodesView: map[string]voteContext{
					"one": {Status: liked},
					"two": {Status: disliked},
				},
			},
			want: true,
		},
		{
			Conflict: Conflict{
				NodesView: map[string]voteContext{
					"one": {Status: liked},
					"two": {},
				},
			},
			want: false,
		},
		{
			Conflict: Conflict{},
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
		Conflict
		want float64
	}{
		{
			Conflict: Conflict{
				NodesView: map[string]voteContext{
					"one": {Status: liked},
					"two": {Status: disliked},
				},
			},
			want: 1,
		},
		{
			Conflict: Conflict{
				NodesView: map[string]voteContext{
					"one": {Status: liked},
					"two": {},
				},
			},
			want: 0.5,
		},
		{
			Conflict: Conflict{},
			want:     0,
		},
	}

	for _, conflictTest := range tests {
		require.Equal(t, conflictTest.want, conflictTest.finalizationStatus())
	}

}
