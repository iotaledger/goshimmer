package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

func TestManager_Get(t *testing.T) {
	const bucketsCount = 20
	const granularity = 3
	baseDir := t.TempDir()

	m := NewManager(context.Background(), WithGranularity(granularity), WithDBProvider(NewDB), WithBaseDir(baseDir), WithMaxOpenDBs(2))

	// Create and write data to buckets.
	{
		for i := granularity; i < bucketsCount; i++ {
			bucket := m.Get(epoch.Index(i), getRealm(i))
			assert.NoError(t, bucket.Set(getKey(i), getValue(i)))
		}

		// Check that internal data structure is correct.
		for i := granularity; i < bucketsCount; i += granularity {
			db, exists := m.dbs.Get(epoch.Index(i))
			require.True(t, exists, "db %d does not exist in data structure", i)
			assert.Equal(t, epoch.Index(i), db.index)
			if i+granularity < bucketsCount {
				assert.Len(t, db.buckets, granularity)
			} else {
				assert.Len(t, db.buckets, bucketsCount-i)
			}
		}

		// Check that folder structure is correct.
		for i := granularity; i < bucketsCount; i++ {
			fileInfo, err := os.Stat(filepath.Join(baseDir, strconv.Itoa(i)))
			if i%granularity == 0 {
				assert.True(t, fileInfo.IsDir())
				require.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, fmt.Sprintf("%d: no such file or directory", i))
			}
		}
	}

	// Read data from buckets.
	{
		for i := granularity; i < bucketsCount; i++ {
			bucket := m.Get(epoch.Index(i), getRealm(i))
			value, err := bucket.Get(getKey(i))
			assert.NoError(t, err)
			assert.Equal(t, getValue(i), value)
		}
	}

	// Flush buckets and check that they are marked healthy.
	{
		for i := granularity; i < bucketsCount; i++ {
			m.Flush(epoch.Index(i))
			bucket := m.getBucket(epoch.Index(i))
			setHealthy, err := bucket.Has(healthKey)
			assert.NoError(t, err)
			assert.True(t, setHealthy)
		}
	}

	// Clean the least recently used DB instances.
	{
		m.cleanLRU()
		assert.Equal(t, 2, m.dbs.Size())
	}

	// Simulate node shutdown.
	m.Shutdown()
	m = nil

	m = NewManager(context.Background(), WithGranularity(granularity), WithDBProvider(NewDB), WithBaseDir(baseDir))
	// Read data from buckets after shutdown (needs to be properly reconstructed from disk).
	{
		for i := granularity; i < bucketsCount; i++ {
			bucket := m.Get(epoch.Index(i), getRealm(i))
			value, err := bucket.Get(getKey(i))
			assert.NoError(t, err)
			assert.Equal(t, getValue(i), value)
		}
	}
}

func getRealm(i int) kvstore.Realm {
	return []byte("realm" + strconv.Itoa(i))
}

func getKey(i int) []byte {
	return []byte("key" + strconv.Itoa(i))
}

func getValue(i int) []byte {
	return []byte("value" + strconv.Itoa(i))
}
