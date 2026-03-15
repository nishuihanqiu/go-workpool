package workpool

import (
	"time"
)

// Logger interface for logging
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// DefaultLogger is a simple logger implementation
type DefaultLogger struct{}

func (d DefaultLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (d DefaultLogger) Info(msg string, keysAndValues ...interface{})  {}
func (d DefaultLogger) Warn(msg string, keysAndValues ...interface{})   {}
func (d DefaultLogger) Error(msg string, keysAndValues ...interface{}) {}

// Config holds the configuration for the worker pool
type Config struct {
	WorkerCount int           // Number of workers, default is CPU cores
	QueueSize   int           // Task queue size, default 1000
	TaskTimeout time.Duration // Task timeout
	PanicHandler func(interface{}) // Panic handler
	Logger      Logger        // Logger interface
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		WorkerCount: 0, // Will be set to CPU cores in New
		QueueSize:   1000,
		Logger:      DefaultLogger{},
	}
}

// Option configures the worker pool
type Option func(*Config)

// WithWorkerCount sets the number of workers
func WithWorkerCount(n int) Option {
	return func(c *Config) {
		c.WorkerCount = n
	}
}

// WithQueueSize sets the queue size
func WithQueueSize(size int) Option {
	return func(c *Config) {
		c.QueueSize = size
	}
}

// WithTaskTimeout sets the task timeout
func WithTaskTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.TaskTimeout = timeout
	}
}

// WithPanicHandler sets the panic handler
func WithPanicHandler(handler func(interface{})) Option {
	return func(c *Config) {
		c.PanicHandler = handler
	}
}

// WithLogger sets the logger
func WithLogger(logger Logger) Option {
	return func(c *Config) {
		c.Logger = logger
	}
}
