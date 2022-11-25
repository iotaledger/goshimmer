package metrics

import "github.com/iotaledger/goshimmer/packages/app/metricscollector"

var TangleMetrics = metricscollector.NewCollection("tangle")

var ConflictMetrics = metricscollector.NewCollection("conflict")
