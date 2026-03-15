package main

import (
	"context"
	"fmt"
	"time"

	"github.com/liliangshan/workpool/workpool"
)

func main() {
	fmt.Println("=== Basic Usage ===")
	basicUsage()

	fmt.Println("\n=== Production Pool Usage ===")
	productionPoolUsage()

	fmt.Println("\n=== Priority Task Usage ===")
	priorityTaskUsage()

	fmt.Println("\n=== Graceful Shutdown Usage ===")
	gracefulShutdownUsage()

	fmt.Println("\n=== Dynamic Scaling Usage ===")
	dynamicScalingUsage()

	fmt.Println("\n=== Metrics Usage ===")
	metricsUsage()

	fmt.Println("\n=== Task with Timeout Usage ===")
	taskTimeoutUsage()
}

// Basic usage with simple API
func basicUsage() {
	pool := workpool.New(workpool.SimpleWithWorkerCount(5))
	pool.Start()

	for i := 0; i < 10; i++ {
		i := i
		pool.Submit(func(ctx context.Context) {
			fmt.Printf("Task %d started\n", i)
			time.Sleep(100 * time.Millisecond)
			fmt.Printf("Task %d completed\n", i)
		})
	}

	pool.Stop()
	fmt.Println("Basic pool stopped")
}

// Production pool with full features
func productionPoolUsage() {
	config := &workpool.Config{
		WorkerCount: 4,
		QueueSize:   1000,
		Logger:      &simpleLogger{},
	}
	pool := workpool.NewPool(config)
	pool.Start()

	for i := 0; i < 20; i++ {
		i := i
		task := workpool.NewTaskWithOptions(
			func(ctx context.Context) {
				fmt.Printf("Production task %d executed\n", i)
			},
			workpool.TaskWithID(fmt.Sprintf("task-%d", i)),
		)
		pool.Submit(task)
	}

	time.Sleep(500 * time.Millisecond)
	metrics := pool.Metrics()
	fmt.Printf("Metrics: %s\n", metrics)

	pool.Stop()
}

// Priority task usage
func priorityTaskUsage() {
	config := &workpool.Config{
		WorkerCount: 3,
		QueueSize:   100,
	}
	pool := workpool.NewPool(config)
	pool.Start()

	// Submit tasks with different priorities
	tasks := []struct {
		id       string
		priority workpool.Priority
	}{
		{"low-1", workpool.PriorityLow},
		{"normal-1", workpool.PriorityNormal},
		{"high-1", workpool.PriorityHigh},
		{"low-2", workpool.PriorityLow},
		{"normal-2", workpool.PriorityNormal},
		{"high-2", workpool.PriorityHigh},
	}

	for _, t := range tasks {
		task := workpool.NewTaskWithOptions(
			func(ctx context.Context) {
				fmt.Printf("Executing task: %s\n", t.id)
			},
			workpool.TaskWithID(t.id),
			workpool.TaskWithPriority(t.priority),
		)
		pool.Submit(task)
	}

	time.Sleep(500 * time.Millisecond)
	pool.Stop()
	fmt.Println("Priority pool stopped")
}

// Graceful shutdown usage
func gracefulShutdownUsage() {
	config := &workpool.Config{
		WorkerCount: 2,
		QueueSize:   100,
	}
	pool := workpool.NewPool(config)
	pool.Start()

	// Submit long-running tasks
	for i := 0; i < 5; i++ {
		i := i
		task := workpool.NewTaskWithOptions(
			func(ctx context.Context) {
				fmt.Printf("Long task %d started\n", i)
				time.Sleep(200 * time.Millisecond)
				fmt.Printf("Long task %d completed\n", i)
			},
			workpool.TaskWithID(fmt.Sprintf("long-task-%d", i)),
		)
		pool.Submit(task)
	}

	// Graceful shutdown with 3 second timeout
	err := pool.GracefulShutdown(3 * time.Second)
	if err != nil {
		fmt.Printf("Graceful shutdown error: %v\n", err)
	} else {
		fmt.Println("Graceful shutdown completed")
	}
}

// Dynamic scaling usage
func dynamicScalingUsage() {
	config := &workpool.Config{
		WorkerCount: 2,
		QueueSize:   100,
	}
	pool := workpool.NewPool(config)
	pool.Start()

	fmt.Printf("Initial worker count: %d\n", pool.WorkerCount())

	// Scale up
	pool.Resize(5)
	fmt.Printf("Scaled up to: %d workers\n", pool.WorkerCount())

	// Submit some tasks
	for i := 0; i < 10; i++ {
		i := i
		pool.Submit(workpool.NewTaskWithOptions(
			func(ctx context.Context) {
				fmt.Printf("Task %d executed by worker\n", i)
			},
			workpool.TaskWithID(fmt.Sprintf("scale-task-%d", i)),
		))
	}

	time.Sleep(300 * time.Millisecond)

	// Scale down
	pool.Resize(2)
	fmt.Printf("Scaled down to: %d workers\n", pool.WorkerCount())

	time.Sleep(300 * time.Millisecond)
	pool.Stop()
	fmt.Println("Dynamic scaling pool stopped")
}

// Metrics usage
func metricsUsage() {
	config := &workpool.Config{
		WorkerCount: 4,
		QueueSize:   100,
	}
	pool := workpool.NewPool(config)
	pool.Start()

	// Submit tasks
	for i := 0; i < 20; i++ {
		i := i
		task := workpool.NewTaskWithOptions(
			func(ctx context.Context) {
				if i%5 == 0 {
					// Simulate failure
					panic("simulated panic")
				}
				time.Sleep(50 * time.Millisecond)
			},
			workpool.TaskWithID(fmt.Sprintf("metrics-task-%d", i)),
		)
		pool.Submit(task)
	}

	time.Sleep(1 * time.Second)

	// Get metrics
	metrics := pool.Metrics()
	fmt.Println("Pool Metrics:")
	fmt.Printf("  Submitted Tasks: %d\n", metrics.SubmittedTasks)
	fmt.Printf("  Completed Tasks: %d\n", metrics.CompletedTasks)
	fmt.Printf("  Failed Tasks: %d\n", metrics.FailedTasks)
	fmt.Printf("  Active Workers: %d\n", metrics.ActiveWorkers)
	fmt.Printf("  Idle Workers: %d\n", metrics.IdleWorkers)
	fmt.Printf("  Queue Size: %d\n", metrics.QueueSize)
	fmt.Printf("  Avg Wait Time: %v\n", metrics.AvgTaskWaitTime)
	fmt.Printf("  Avg Exec Time: %v\n", metrics.AvgTaskExecTime)

	pool.Stop()
}

// Task with timeout usage
func taskTimeoutUsage() {
	config := &workpool.Config{
		WorkerCount: 2,
		QueueSize:   100,
	}
	pool := workpool.NewPool(config)
	pool.Start()

	// Task that completes in time
	task1 := workpool.NewTaskWithOptions(
		func(ctx context.Context) {
			fmt.Println("Quick task executed")
		},
		workpool.TaskWithID("quick-task"),
		workpool.TaskWithTimeout(2*time.Second),
	)

	// Task that times out
	task2 := workpool.NewTaskWithOptions(
		func(ctx context.Context) {
			time.Sleep(3 * time.Second)
			fmt.Println("Slow task executed (should timeout)")
		},
		workpool.TaskWithID("slow-task"),
		workpool.TaskWithTimeout(1*time.Second),
	)

	pool.Submit(task1)
	pool.Submit(task2)

	time.Sleep(2 * time.Second)
	pool.Stop()
	fmt.Println("Timeout pool stopped")
}

// simpleLogger is a simple logger implementation
type simpleLogger struct{}

func (l *simpleLogger) Debug(msg string, keysAndValues ...interface{}) {
	fmt.Printf("[DEBUG] %s\n", msg)
}

func (l *simpleLogger) Info(msg string, keysAndValues ...interface{}) {
	fmt.Printf("[INFO] %s\n", msg)
}

func (l *simpleLogger) Warn(msg string, keysAndValues ...interface{}) {
	fmt.Printf("[WARN] %s\n", msg)
}

func (l *simpleLogger) Error(msg string, keysAndValues ...interface{}) {
	fmt.Printf("[ERROR] %s\n", msg)
}
