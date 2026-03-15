package workpool

import (
	"runtime"
	"sync"
	"time"
)

// worker represents a worker that processes tasks
type worker struct {
	id      int
	pool    *Pool
	stopped chan struct{}
	mu      sync.Mutex
}

// newWorker creates a new worker
func newWorker(id int, pool *Pool) *worker {
	return &worker{
		id:      id,
		pool:    pool,
		stopped: make(chan struct{}),
	}
}

// run starts the worker loop
func (w *worker) run(wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		// Check if worker is stopped
		select {
		case <-w.stopped:
			return
		default:
		}

		// Check if pool is stopped
		select {
		case <-w.pool.ctx.Done():
			return
		case <-w.stopped:
			return
		default:
		}

		// Get task from queue with non-blocking pop first
		task := w.pool.taskQueue.PopNonBlocking()

		if task == nil {
			// No task available, wait a bit
			time.Sleep(10 * time.Millisecond)
			continue
		}

		// Execute task
		w.execute(task)
	}
}

// execute executes a task
func (w *worker) execute(task *Task) {
	startTime := time.Now()

	// Update metrics - mark worker as busy
	w.pool.metrics.SetIdleWorkers(w.pool.metrics.IdleWorkers - 1)

	// Execute with panic handling
	defer func() {
		if r := recover(); r != nil {
			if w.pool.config.PanicHandler != nil {
				w.pool.config.PanicHandler(r)
			} else {
				defaultPanicHandler(r)
			}
			w.pool.metrics.IncrementFailed()
		}

		// Update metrics - mark worker as idle
		w.pool.metrics.SetIdleWorkers(w.pool.metrics.IdleWorkers + 1)
		w.pool.metrics.SetQueueSize(int64(w.pool.taskQueue.Len()))
		w.pool.metrics.RecordExecTime(time.Since(startTime))
		w.pool.metrics.IncrementCompleted()
	}()

	_, err := task.Execute(w.pool.ctx)
	if err != nil {
		w.pool.metrics.IncrementFailed()
		if w.pool.config.Logger != nil {
			w.pool.config.Logger.Error("Task execution failed",
				"task_id", task.ID,
				"error", err.Error())
		}
	}
}

// stop stops the worker
func (w *worker) stop() {
	close(w.stopped)
}

// defaultPanicHandler is the default panic handler
func defaultPanicHandler(r interface{}) {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	panicMsg := string(buf[:n])
	// In production, you would log this
	_ = panicMsg
}
