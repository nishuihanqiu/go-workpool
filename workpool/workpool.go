package workpool

import (
	"context"
	"runtime"
	"sync"
	"time"
)

// SimpleTask represents a simple task to be executed by the worker pool
type SimpleTask func(ctx context.Context)

// SimpleOption configures the simple worker pool
type SimpleOption func(*SimplePool)

// WithWorkerCount sets the number of workers
func SimpleWithWorkerCount(n int) SimpleOption {
	return func(p *SimplePool) {
		p.workers = n
	}
}

// SimplePool represents a simple worker pool
type SimplePool struct {
	workers    int
	taskChan   chan SimpleTask
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

// New creates a new simple worker pool
func New(opts ...SimpleOption) *SimplePool {
	p := &SimplePool{
		workers:  runtime.NumCPU(),
		taskChan: make(chan SimpleTask),
	}

	for _, opt := range opts {
		opt(p)
	}

	p.ctx, p.cancel = context.WithCancel(context.Background())

	return p
}

// Start starts the simple worker pool
func (p *SimplePool) Start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
}

// worker is the worker goroutine
func (p *SimplePool) worker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		case task, ok := <-p.taskChan:
			if !ok {
				return
			}
			task(p.ctx)
		}
	}
}

// Submit submits a task to the simple pool
func (p *SimplePool) Submit(task SimpleTask) {
	select {
	case <-p.ctx.Done():
		return
	case p.taskChan <- task:
	}
}

// Stop stops the simple worker pool and waits for all workers to finish
func (p *SimplePool) Stop() {
	p.cancel()
	p.wg.Wait()
	close(p.taskChan)
}

// SimplePoolWithResult represents a simple worker pool that can return results
type SimplePoolWithResult struct {
	*SimplePool
	resultChan chan interface{}
}

// NewWithResult creates a new simple worker pool that can return results
func NewWithResult(workers int) *SimplePoolWithResult {
	p := New(SimpleWithWorkerCount(workers))
	return &SimplePoolWithResult{
		SimplePool:  p,
		resultChan: make(chan interface{}, workers),
	}
}

// SubmitWithResult submits a task that returns a result
func (p *SimplePoolWithResult) SubmitWithResult(task func() interface{}) {
	p.Submit(func(ctx context.Context) {
		p.resultChan <- task()
	})
}

// Result returns a result from the pool
func (p *SimplePoolWithResult) Result() <-chan interface{} {
	return p.resultChan
}

// ============ Production-grade API ============

// NewPool creates a new production-grade pool
// Use Config options to configure the pool
func NewPool(cfg *Config) *Pool {
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = runtime.NumCPU()
	}

	p := &Pool{
		config:      cfg,
		taskQueue:   NewTaskQueue(cfg.QueueSize),
		workerCount: cfg.WorkerCount,
		metrics:     NewMetrics(),
		strategy:    Block,
		state:      PoolStateStopped,
	}

	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.workers = make([]*worker, 0, p.workerCount)

	return p
}

// SubmitWithTimeout submits a task with timeout
func (p *SimplePool) SubmitWithTimeout(task SimpleTask, timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		p.Submit(task)
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return context.DeadlineExceeded
	}
}

// GracefulShutdown waits for pending tasks to complete
func (p *SimplePool) GracefulShutdown(timeout time.Duration) error {
	p.cancel()

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return context.DeadlineExceeded
	}
}
