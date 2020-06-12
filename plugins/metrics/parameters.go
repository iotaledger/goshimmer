package metrics

import "time"

const (
	// should always be 1 second
	MPSMeasurementInterval = 1 * time.Second
	TPSMeasurementInterval = 1 * time.Second
	// can be adjusted as wished
	MessageTipsMeasurementInterval = 1 * time.Second
	ValueTipsMeasurementInterval   = 1 * time.Second
	CPUUsageMeasurementInterval    = 1 * time.Second
	MemUsageMeasurementInterval    = 1 * time.Second
	SyncedMeasurementInterval      = 1 * time.Second
)
