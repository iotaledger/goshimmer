package database

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/serializer/v2/byteutils"
)

func TestManager_Get(t *testing.T) {
	const bucketsCount = 20
	const granularity = 3
	baseDir := t.TempDir()

	m := NewManager(1, WithGranularity(granularity), WithDBProvider(NewDB), WithBaseDir(baseDir), WithMaxOpenDBs(2))

	dbSize := m.PrunableStorageSize()

	// Create and write data to buckets.
	{
		for i := granularity; i < bucketsCount; i++ {
			bucket := m.Get(slot.Index(i), getRealm(i))
			assert.NoError(t, bucket.Set(getKey(i), getValue(i)))
		}

		// Check that folder structure is correct.
		for i := granularity; i < bucketsCount; i++ {
			fileInfo, err := os.Stat(filepath.Join(baseDir, "pruned", strconv.Itoa(i)))
			if i%granularity == 0 {
				require.NoError(t, err)
				assert.True(t, fileInfo.IsDir())
			} else {
				assert.ErrorContains(t, err, fmt.Sprintf("%d: no such file or directory", i))
			}
		}
	}

	// dbSize should have increased with new buckets
	assert.Greater(t, m.PrunableStorageSize(), dbSize)

	// Read data from buckets.
	{
		for i := granularity; i < bucketsCount; i++ {
			bucket := m.Get(slot.Index(i), getRealm(i))
			value, err := bucket.Get(getKey(i))
			assert.NoError(t, err)
			assert.Equal(t, getValue(i), value)
		}
	}

	// Flush buckets and check that they are marked healthy.
	{
		for i := granularity; i < bucketsCount; i++ {
			m.Flush(slot.Index(i))
			bucket := m.getBucket(slot.Index(i))
			setHealthy, err := bucket.Has(healthKey)
			assert.NoError(t, err)
			assert.True(t, setHealthy)
		}
	}

	// Check that only expected values + health is in each bucket.
	{
		expected := make([][]byte, 0)
		for i := granularity; i < bucketsCount; i++ {
			bucketData := byteutils.ConcatBytes(getRealm(i), getRealm(i), getKey(i), getValue(i))
			healthData := byteutils.ConcatBytes(getRealm(i), healthKey, []byte{1})
			expected = append(expected, bucketData, healthData)
		}

		actual := make([][]byte, 0)
		for i := granularity; i < bucketsCount; i += granularity {
			db := m.getDBInstance(slot.Index(i))
			require.NoError(t, db.store.Iterate(kvstore.EmptyPrefix, func(key kvstore.Key, value kvstore.Value) bool {
				actual = append(actual, byteutils.ConcatBytes(key, value))
				return true
			}))
		}

		assert.ElementsMatch(t, expected, actual)
	}

	dbSize = m.PrunableStorageSize()

	// Prune some stuff.
	expectedFirstBucket := slot.Index(5) + 1
	{
		// Pruning a slot that is not dividable by granularity does not actually prune.
		m.PruneUntilSlot(4)
		assert.EqualValues(t, 2, m.maxPruned)

		m.PruneUntilSlot(7)
		assert.EqualValues(t, 5, m.maxPruned)

		// Check that folder structure is correct and everything below expectedFirstBucket is deleted.
		for i := granularity; i < bucketsCount; i++ {
			fileInfo, err := os.Stat(filepath.Join(baseDir, "pruned", strconv.Itoa(i)))
			if i < int(expectedFirstBucket) {
				assert.ErrorContains(t, err, fmt.Sprintf("%d: no such file or directory", i))
			} else if i%granularity == 0 {
				require.NoError(t, err)
				assert.True(t, fileInfo.IsDir())
			} else {
				assert.ErrorContains(t, err, fmt.Sprintf("%d: no such file or directory", i))
			}
		}
	}

	// After pruning the dbSize should be smaller
	assert.Less(t, m.PrunableStorageSize(), dbSize)

	// Insert into buckets but DO NOT mark as clean. When restoring from disk they should not be healthy and thus be
	// deleted.
	totalBucketCount := bucketsCount + 10
	for i := bucketsCount; i < totalBucketCount; i++ {
		bucket := m.Get(slot.Index(i), getRealm(i))
		assert.NoError(t, bucket.Set(getKey(i), getValue(i)))
	}

	// Simulate node shutdown.
	m.Shutdown()
	m = nil

	m = NewManager(1, WithGranularity(granularity), WithDBProvider(NewDB), WithBaseDir(baseDir))
	// Read data from buckets after shutdown (needs to be properly reconstructed from disk).
	{
		for i := int(expectedFirstBucket); i < bucketsCount; i++ {
			bucket := m.Get(slot.Index(i), getRealm(i))
			value, err := bucket.Get(getKey(i))
			assert.NoError(t, err)
			assert.Equal(t, getValue(i), value, "slot %d: expected %+v but got %+v", i, getValue(i), value)
		}
	}

	latestBucketIndex := m.RestoreFromDisk()
	assert.EqualValues(t, bucketsCount-1, latestBucketIndex)

	// Check that folder structure is correct. Everything above bucketsCount-1 was unhealthy -> db files should be deleted.
	{
		for i := int(expectedFirstBucket); i < totalBucketCount; i++ {
			fileInfo, err := os.Stat(filepath.Join(baseDir, "pruned", strconv.Itoa(i)))
			if i%granularity == 0 && i < bucketsCount {
				require.NoError(t, err)
				assert.True(t, fileInfo.IsDir())
			} else {
				assert.ErrorContains(t, err, fmt.Sprintf("%d: no such file or directory", i))
			}
		}
	}

	// Check that there's no unhealthy bucket in the highest db instance.
	{
		latestDBIndex := m.computeDBBaseIndex(latestBucketIndex)
		expected := make([][]byte, 0)
		for i := latestDBIndex + granularity - 1; i >= latestDBIndex; i-- {
			if i > latestBucketIndex {
				continue
			}
			j := int(i)
			bucketData := byteutils.ConcatBytes(getRealm(j), getRealm(j), getKey(j), getValue(j))
			healthData := byteutils.ConcatBytes(getRealm(j), healthKey, []byte{1})
			expected = append(expected, bucketData, healthData)
		}

		actual := make([][]byte, 0)
		for i := latestDBIndex + granularity - 1; i > latestBucketIndex; i-- {
			db := m.getDBInstance(i)
			require.NoError(t, db.store.Iterate(kvstore.EmptyPrefix, func(key kvstore.Key, value kvstore.Value) bool {
				actual = append(actual, byteutils.ConcatBytes(key, value))
				return true
			}))
		}

		assert.ElementsMatch(t, expected, actual)
	}

	// We pruned until slot baseIndex granularity. Thus this should be the pruned slot baseIndex after restoring.
	assert.Equal(t, expectedFirstBucket-1, m.maxPruned)
}

func getRealm(i int) kvstore.Realm {
	return indexToRealm(slot.Index(i))
}

func getKey(i int) []byte {
	return []byte("key" + strconv.Itoa(i))
}

func getValue(i int) []byte {
	return []byte("value" + strconv.Itoa(i))
}
