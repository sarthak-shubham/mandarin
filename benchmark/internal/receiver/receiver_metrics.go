package receiver

import (
	"sync/atomic"
	"time"
)

type ReceiverMetrics struct {
	totalReceived atomic.Int64
	successCount  atomic.Int64
	failureCount  atomic.Int64
	firstArrival  atomic.Int64
	lastArrival   atomic.Int64
}

type MetricsSnapshot struct {
	TotalReceived int64         `json:"total_received"`
	SuccessCount  int64         `json:"success_count"`
	FailureCount  int64         `json:"failure_count"`
	Duration      time.Duration `json:"duration"`
	ThroughputRPS float64       `json:"throughput_rps"`
}

func NewReceiverMetrics() *ReceiverMetrics {
	m := &ReceiverMetrics{}
	return m
}

func (m *ReceiverMetrics) RecordDelivery(success bool) {
	now := time.Now().UnixNano()

	m.totalReceived.Add(1)
	if success {
		m.successCount.Add(1)
	} else {
		m.failureCount.Add(1)
	}

	m.firstArrival.CompareAndSwap(0, now)
	m.lastArrival.Store(now)
}

func (m *ReceiverMetrics) Snapshot() MetricsSnapshot {
	total := m.totalReceived.Load()
	success := m.successCount.Load()
	failure := m.failureCount.Load()
	first := m.firstArrival.Load()
	last := m.lastArrival.Load()

	var duration time.Duration
	if first > 0 && last >= first {
		duration = time.Duration(last - first)
	}

	var rps float64
	if duration.Seconds() > 0 && total > 0 {
		rps = float64(total) / duration.Seconds()
	}

	snap := MetricsSnapshot{
		TotalReceived: total,
		SuccessCount:  success,
		FailureCount:  failure,
		Duration:      duration,
		ThroughputRPS: rps,
	}
	return snap
}

func (m *ReceiverMetrics) Reset() {
	m.totalReceived.Store(0)
	m.successCount.Store(0)
	m.failureCount.Store(0)
	m.firstArrival.Store(0)
	m.lastArrival.Store(0)
}
