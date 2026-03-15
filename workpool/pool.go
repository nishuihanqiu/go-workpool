package workpool

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// PoolInterface defines the interface for worker pool
type PoolInterface interface {
	Start()
	Stop()
	Submit(task *Task)
	SubmitWait(task *Task) error
	WorkerCount() int
	QueueCapacity() int
	Metrics() *MetricsSnapshot
	Resize(n int)
	GracefulShutdown(timeout time.Duration) error
}

// SaturationStrategy defines the behavior when the pool is saturated
type SaturationStrategy int

const (
	// Abort returns error when queue is full
	Abort SaturationStrategy = iota
	// DropOldest drops the oldest task in queue
	DropOldest
	// Block blocks until space is available
	Block
	// CallerRuns runs the task in the caller's goroutine
	CallerRuns
)

// Pool represents a worker pool
type Pool struct {
	config         *Config
	taskQueue      *TaskQueue
	workers        []*worker
	workerCount    int
	metrics        *Metrics
	wg             sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
	mu             sync.RWMutex
	state          PoolState
	strategy       SaturationStrategy
}

// PoolState represents the state of the pool
type PoolState int

const (
	PoolStateRunning PoolState = iota
	PoolStateStopping
	PoolStateStopped
)

func (s PoolState) String() string {
	switch s {
	case PoolStateRunning:
		return "Running"
	case PoolStateStopping:
		return "Stopping"
	case PoolStateStopped:
		return "Stopped"
	default:
		return "Unknown"
	}
}

// NewPro creates a new production worker pool with options
func NewPro(opts ...Option) *Pool {
	config := DefaultConfig()
	for _, opt := range opts {
		opt(config)
	}

	if config.WorkerCount <= 0 {
		config.WorkerCount = runtime.NumCPU()
	}

	p := &Pool{
		config:      config,
		taskQueue:   NewTaskQueue(config.QueueSize),
		workerCount: config.WorkerCount,
		metrics:     NewMetrics(),
		strategy:    Block,
	}

	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.workers = make([]*worker, 0, p.workerCount)

	return p
}

// WithSaturationStrategy sets the saturation strategy
func WithSaturationStrategy(s SaturationStrategy) Option {
	return func(c *Config) {
		// This will be handled in Pool
	}
}

// Start starts the worker pool
func (p *Pool) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Only start from stopped state
	if p.state == PoolStateRunning || p.state == PoolStateStopping {
		return
	}

	p.state = PoolStateRunning
	p.ctx, p.cancel = context.WithCancel(context.Background())

	for i := 0; i < p.workerCount; i++ {
		w := newWorker(i, p)
		p.workers = append(p.workers, w)
		p.wg.Add(1)
		go w.run(&p.wg)
	}

	p.metrics.SetActiveWorkers(int64(p.workerCount))
	p.metrics.SetIdleWorkers(int64(p.workerCount))
}

// Stop stops the worker pool
func (p *Pool) Stop() {
	p.mu.Lock()
	if p.state != PoolStateRunning {
		p.mu.Unlock()
		return
	}
	p.state = PoolStateStopping
	p.cancel()
	p.mu.Unlock()

	p.wg.Wait()

	p.mu.Lock()
	p.state = PoolStateStopped
	p.metrics.SetActiveWorkers(0)
	p.metrics.SetIdleWorkers(0)
	p.mu.Unlock()

	closeTaskChannels(p.taskQueue)
}

// Submit submits a task to the pool
func (p *Pool) Submit(task *Task) {
	if p.ctx.Err() != nil {
		return
	}

	p.metrics.IncrementSubmitted()
	p.metrics.RecordWaitTime(time.Since(task.CreatedAt))
	p.metrics.SetQueueSize(int64(p.taskQueue.Len()))

	switch p.strategy {
	case Abort:
		select {
		case <-p.ctx.Done():
			return
		default:
			if !p.taskQueue.Push(task) {
				p.metrics.IncrementFailed()
			}
		}
	case DropOldest:
		if !p.taskQueue.Push(task) {
			// Drop oldest and retry
			oldest := p.taskQueue.PopNonBlocking()
			if oldest != nil {
				p.taskQueue.Push(task)
			} else {
				p.taskQueue.Push(task)
			}
		}
	case Block:
		p.taskQueue.PushBlocking(task)
	case CallerRuns:
		select {
		case <-p.ctx.Done():
			return
		default:
			if !p.taskQueue.Push(task) {
				// Run in caller goroutine
				go task.Execute(p.ctx)
			}
		}
	}
}

// SubmitWait submits a task and waits for completion
func (p *Pool) SubmitWait(task *Task) error {
	done := make(chan struct{})
	go func() {
		p.Submit(task)
		close(done)
	}()

	select {
	case <-done:
	case <-p.ctx.Done():
		return p.ctx.Err()
	}

	// Wait for task completion
	<-task.done
	return task.err
}

// WorkerCount returns the number of workers
func (p *Pool) WorkerCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.workerCount
}

// QueueCapacity returns the queue capacity
func (p *Pool) QueueCapacity() int {
	return p.config.QueueSize
}

// Metrics returns the metrics snapshot
func (p *Pool) Metrics() *MetricsSnapshot {
	return p.metrics.Snapshot()
}

// Resize resizes the worker pool
func (p *Pool) Resize(n int) {
	if n <= 0 {
		n = runtime.NumCPU()
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	current := len(p.workers)
	if n == current {
		return
	}

	if n < current {
		// Remove workers
		remove := current - n
		for i := 0; i < remove; i++ {
			if len(p.workers) > 0 {
				w := p.workers[len(p.workers)-1]
				p.workers = p.workers[:len(p.workers)-1]
				go func() {
					w.stop()
				}()
			}
		}
		p.metrics.SetActiveWorkers(int64(len(p.workers)))
	} else {
		// Add workers
		for i := current; i < n; i++ {
			w := newWorker(i, p)
			p.workers = append(p.workers, w)
			p.wg.Add(1)
			go w.run(&p.wg)
		}
	}

	p.workerCount = n
}

// GracefulShutdown waits for all tasks to complete with timeout
func (p *Pool) GracefulShutdown(timeout time.Duration) error {
	p.mu.Lock()
	if p.state != PoolStateRunning {
		p.mu.Unlock()
		return nil
	}
	p.state = PoolStateStopping
	p.cancel()
	p.mu.Unlock()

	// Wait for workers to finish
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.mu.Lock()
		p.state = PoolStateStopped
		p.mu.Unlock()
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("graceful shutdown timeout after %v", timeout)
	}
}

// GetTaskQueue returns the task queue
func (p *Pool) GetTaskQueue() *TaskQueue {
	return p.taskQueue
}

// GetContext returns the context
func (p *Pool) GetContext() context.Context {
	return p.ctx
}

// GetConfig returns the config
func (p *Pool) GetConfig() *Config {
	return p.config
}

// GetMetrics returns the metrics
func (p *Pool) GetMetrics() *Metrics {
	return p.metrics
}

// closeTaskChannels closes task channels
func closeTaskChannels(q *TaskQueue) {
	close(q.high)
	close(q.normal)
	close(q.low)
}

// SetSaturationStrategy sets the saturation strategy
func (p *Pool) SetSaturationStrategy(s SaturationStrategy) {
	p.strategy = s
}
