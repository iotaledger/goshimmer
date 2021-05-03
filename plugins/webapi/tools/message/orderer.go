package message

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

// OrdererDebugHandler runs the tangle.Orderer analysis which consists of printing all the time buckets.
func OrdererDebugHandler(c echo.Context) error {
	buckets, bucketsMap := messagelayer.Tangle().Storage.AllBucketMessageIDsBuckets()

	sort.Slice(buckets, func(i, j int) bool { return buckets[i] < buckets[j] })

	for i, b := range buckets {
		fmt.Println(i, bucketsMap[b], time.Unix(b, 0))
		if i > 100 {
			break
		}
	}

	return c.NoContent(http.StatusNoContent)
}
