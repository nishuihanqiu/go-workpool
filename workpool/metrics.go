package workpool

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics holds the pool metrics
type Metrics struct {
	mu               sync.RWMutex
	SubmittedTasks   int64
	CompletedTasks   int64
	FailedTasks      int64
	ActiveWorkers   int64
	IdleWorkers     int64
	QueueSize       int64
	TotalWaitTime   time.Duration
	TotalExecTime   time.Duration
	TaskWaitTimes   []time.Duration
	TaskExecTimes   []time.Duration
	lastTaskWait    time.Duration
	lastTaskExec    time.Duration
}

// NewMetrics creates a new metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		TaskWaitTimes: make([]time.Duration, 0, 1000),
		TaskExecTimes: make([]time.Duration, 0, 1000),
	}
}

// IncrementSubmitted increments submitted tasks counter
func (m *Metrics) IncrementSubmitted() {
	atomic.AddInt64(&m.SubmittedTasks, 1)
}

// IncrementCompleted increments completed tasks counter
func (m *Metrics) IncrementCompleted() {
	atomic.AddInt64(&m.CompletedTasks, 1)
}

// IncrementFailed increments failed tasks counter
func (m *Metrics) IncrementFailed() {
	atomic.AddInt64(&m.FailedTasks, 1)
}

// SetActiveWorkers sets the number of active workers
func (m *Metrics) SetActiveWorkers(n int64) {
	atomic.StoreInt64(&m.ActiveWorkers, n)
}

// SetIdleWorkers sets the number of idle workers
func (m *Metrics) SetIdleWorkers(n int64) {
	atomic.StoreInt64(&m.IdleWorkers, n)
}

// SetQueueSize sets the current queue size
func (m *Metrics) SetQueueSize(n int64) {
	atomic.StoreInt64(&m.QueueSize, n)
}

// RecordWaitTime records task wait time
func (m *Metrics) RecordWaitTime(d time.Duration) {
	m.mu.Lock()
	m.lastTaskWait = d
	m.TotalWaitTime += d
	if len(m.TaskWaitTimes) < 1000 {
		m.TaskWaitTimes = append(m.TaskWaitTimes, d)
	}
	m.mu.Unlock()
}

// RecordExecTime records task execution time
func (m *Metrics) RecordExecTime(d time.Duration) {
	m.mu.Lock()
	m.lastTaskExec = d
	m.TotalExecTime += d
	if len(m.TaskExecTimes) < 1000 {
		m.TaskExecTimes = append(m.TaskExecTimes, d)
	}
	m.mu.Unlock()
}

// Snapshot returns a snapshot of the metrics
func (m *Metrics) Snapshot() *MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var avgWait, avgExec time.Duration
	if m.CompletedTasks > 0 {
		avgWait = m.TotalWaitTime / time.Duration(m.CompletedTasks)
		avgExec = m.TotalExecTime / time.Duration(m.CompletedTasks)
	}

	return &MetricsSnapshot{
		SubmittedTasks:  atomic.LoadInt64(&m.SubmittedTasks),
		CompletedTasks:  atomic.LoadInt64(&m.CompletedTasks),
		FailedTasks:     atomic.LoadInt64(&m.FailedTasks),
		ActiveWorkers:   atomic.LoadInt64(&m.ActiveWorkers),
		IdleWorkers:    atomic.LoadInt64(&m.IdleWorkers),
		QueueSize:       atomic.LoadInt64(&m.QueueSize),
		AvgTaskWaitTime: avgWait,
		AvgTaskExecTime: avgExec,
		LastTaskWaitTime: m.lastTaskWait,
		LastTaskExecTime: m.lastTaskExec,
	}
}

// MetricsSnapshot represents a snapshot of metrics
type MetricsSnapshot struct {
	SubmittedTasks  int64
	CompletedTasks  int64
	FailedTasks     int64
	ActiveWorkers   int64
	IdleWorkers    int64
	QueueSize       int64
	AvgTaskWaitTime time.Duration
	AvgTaskExecTime time.Duration
	LastTaskWaitTime time.Duration
	LastTaskExecTime time.Duration
}

// String returns string representation of metrics
func (m *MetricsSnapshot) String() string {
	return "Metrics{" +
		"Submitted: " + itoa(m.SubmittedTasks) +
		", Completed: " + itoa(m.CompletedTasks) +
		", Failed: " + itoa(m.FailedTasks) +
		", ActiveWorkers: " + itoa(m.ActiveWorkers) +
		", IdleWorkers: " + itoa(m.IdleWorkers) +
		", QueueSize: " + itoa(m.QueueSize) +
		", AvgWaitTime: " + m.AvgTaskWaitTime.String() +
		", AvgExecTime: " + m.AvgTaskExecTime.String() +
		"}"
}

// itoa converts integer to string
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var s string
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
