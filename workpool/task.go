package workpool

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Priority represents task priority
type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
)

func (p Priority) String() string {
	switch p {
	case PriorityHigh:
		return "HIGH"
	case PriorityNormal:
		return "NORMAL"
	case PriorityLow:
		return "LOW"
	default:
		return "UNKNOWN"
	}
}

// TaskFunc represents a task function
type TaskFunc func(ctx context.Context)

// Task represents a task to be executed by the worker pool
type Task struct {
	ID        string
	Fn        TaskFunc
	Priority  Priority
	Timeout   time.Duration
	CreatedAt time.Time

	result   interface{}
	err      error
	done     chan struct{}
	mu       sync.Mutex
}

// NewTask creates a new task
func NewTask(fn TaskFunc) *Task {
	return &Task{
		Fn:        fn,
		Priority:  PriorityNormal,
		CreatedAt: time.Now(),
		done:      make(chan struct{}),
	}
}

// TaskWithID creates a new task with ID
func TaskWithID(id string) func(*Task) {
	return func(t *Task) {
		t.ID = id
	}
}

// TaskWithPriority creates a new task with priority
func TaskWithPriority(p Priority) func(*Task) {
	return func(t *Task) {
		t.Priority = p
	}
}

// TaskWithTimeout creates a new task with timeout
func TaskWithTimeout(timeout time.Duration) func(*Task) {
	return func(t *Task) {
		t.Timeout = timeout
	}
}

// NewTaskWithOptions creates a new task with options
func NewTaskWithOptions(fn TaskFunc, opts ...func(*Task)) *Task {
	task := NewTask(fn)
	for _, opt := range opts {
		opt(task)
	}
	if task.ID == "" {
		task.ID = fmt.Sprintf("task-%d", atomic.AddInt64(&taskIDCounter, 1))
	}
	return task
}

var taskIDCounter int64

// Execute executes the task with optional timeout
func (t *Task) Execute(ctx context.Context) (interface{}, error) {
	t.mu.Lock()
	if t.done == nil {
		t.done = make(chan struct{})
	}
	done := t.done
	t.mu.Unlock()

	// If already executed, return cached result
	select {
	case <-done:
		return t.result, t.err
	default:
	}

	var taskCtx context.Context
	var cancel context.CancelFunc

	if t.Timeout > 0 {
		taskCtx, cancel = context.WithTimeout(ctx, t.Timeout)
		defer cancel()
	} else {
		taskCtx = ctx
	}

	// Execute in goroutine to handle timeout
	doneChan := make(chan struct{})
	go func() {
		defer close(doneChan)
		defer func() {
			if r := recover(); r != nil {
				t.err = fmt.Errorf("panic: %v", r)
			}
		}()
		t.Fn(taskCtx)
	}()

	// Wait for completion or timeout
	select {
	case <-doneChan:
		t.mu.Lock()
		close(t.done)
		t.result = nil
		t.err = nil
		t.mu.Unlock()
		return t.result, t.err
	case <-taskCtx.Done():
		t.mu.Lock()
		close(t.done)
		t.err = taskCtx.Err()
		t.mu.Unlock()
		return nil, t.err
	}
}

// String returns string representation of task
func (t *Task) String() string {
	return fmt.Sprintf("Task{id=%s, priority=%s}", t.ID, t.Priority)
}

// TaskQueue is a priority queue for tasks
type TaskQueue struct {
	high   chan *Task
	normal chan *Task
	low    chan *Task
}

// NewTaskQueue creates a new priority queue
func NewTaskQueue(size int) *TaskQueue {
	return &TaskQueue{
		high:   make(chan *Task, size/4),
		normal: make(chan *Task, size/2),
		low:    make(chan *Task, size/4),
	}
}

// Push adds a task to the queue
func (q *TaskQueue) Push(t *Task) bool {
	switch t.Priority {
	case PriorityHigh:
		select {
		case q.high <- t:
			return true
		default:
			return false
		}
	case PriorityNormal:
		select {
		case q.normal <- t:
			return true
		default:
			return false
		}
	case PriorityLow:
		select {
		case q.low <- t:
			return true
		default:
			return false
		}
	default:
		select {
		case q.normal <- t:
			return true
		default:
			return false
		}
	}
}

// PushBlocking adds a task to the queue (blocking)
func (q *TaskQueue) PushBlocking(t *Task) {
	switch t.Priority {
	case PriorityHigh:
		q.high <- t
	case PriorityNormal:
		q.normal <- t
	case PriorityLow:
		q.low <- t
	default:
		q.normal <- t
	}
}

// Pop removes a task from the queue (blocking)
func (q *TaskQueue) Pop() *Task {
	select {
	case t := <-q.high:
		return t
	case t := <-q.normal:
		return t
	case t := <-q.low:
		return t
	}
}

// PopNonBlocking removes a task from the queue (non-blocking)
func (q *TaskQueue) PopNonBlocking() *Task {
	select {
	case t := <-q.high:
		return t
	default:
		select {
		case t := <-q.normal:
			return t
		default:
			select {
			case t := <-q.low:
				return t
			default:
				return nil
			}
		}
	}
}

// Len returns the number of tasks in the queue
func (q *TaskQueue) Len() int {
	return len(q.high) + len(q.normal) + len(q.low)
}

// IsEmpty returns true if the queue is empty
func (q *TaskQueue) IsEmpty() bool {
	return len(q.high) == 0 && len(q.normal) == 0 && len(q.low) == 0
}
