package service

import (
	"sync/atomic"
)

type calendarSyncMetrics struct {
	jobsQueued           uint64
	jobsClaimed          uint64
	jobsSucceeded        uint64
	jobsFailed           uint64
	reauthRequiredEvents uint64
}

var calendarMetrics calendarSyncMetrics

func calendarMetricJobQueued() {
	atomic.AddUint64(&calendarMetrics.jobsQueued, 1)
}

func calendarMetricJobClaimed(count int) {
	if count <= 0 {
		return
	}
	atomic.AddUint64(&calendarMetrics.jobsClaimed, uint64(count))
}

func calendarMetricJobSucceeded() {
	atomic.AddUint64(&calendarMetrics.jobsSucceeded, 1)
}

func calendarMetricJobFailed() {
	atomic.AddUint64(&calendarMetrics.jobsFailed, 1)
}

func calendarMetricReauthRequired() {
	atomic.AddUint64(&calendarMetrics.reauthRequiredEvents, 1)
}

func calendarSyncMetricsSnapshot() map[string]uint64 {
	return map[string]uint64{
		"jobs_queued":              atomic.LoadUint64(&calendarMetrics.jobsQueued),
		"jobs_claimed":             atomic.LoadUint64(&calendarMetrics.jobsClaimed),
		"jobs_succeeded":           atomic.LoadUint64(&calendarMetrics.jobsSucceeded),
		"jobs_failed":              atomic.LoadUint64(&calendarMetrics.jobsFailed),
		"reauthorization_required": atomic.LoadUint64(&calendarMetrics.reauthRequiredEvents),
	}
}
