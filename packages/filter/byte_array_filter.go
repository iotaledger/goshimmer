package filter

import (
    "github.com/iotaledger/goshimmer/packages/typeconversion"
    "sync"
)

type ByteArrayFilter struct {
    byteArrays      [][]byte
    byteArraysByKey map[string]bool
    size            int
    mutex           sync.RWMutex
}

func NewByteArrayFilter(size int) *ByteArrayFilter {
    return &ByteArrayFilter{
        byteArrays:      make([][]byte, 0, size),
        byteArraysByKey: make(map[string]bool, size),
        size:            size,
    }
}

func (filter *ByteArrayFilter) Contains(byteArray []byte) bool {
    filter.mutex.RLock()
    defer filter.mutex.RUnlock()

    _, exists := filter.byteArraysByKey[typeconversion.BytesToString(byteArray)]

    return exists
}

func (filter *ByteArrayFilter) Add(byteArray []byte) bool {
    key := typeconversion.BytesToString(byteArray)

    filter.mutex.Lock()
    defer filter.mutex.Unlock()

    if _, exists := filter.byteArraysByKey[key]; !exists {
        if len(filter.byteArrays) == filter.size {
            delete(filter.byteArraysByKey, typeconversion.BytesToString(filter.byteArrays[0]))

            filter.byteArrays = append(filter.byteArrays[1:], byteArray)
        } else {
            filter.byteArrays = append(filter.byteArrays, byteArray)
        }

        filter.byteArraysByKey[key] = true

        return true
    } else {
        return false
    }
}
