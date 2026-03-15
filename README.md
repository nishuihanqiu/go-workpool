# Go WorkPool

一个生产级别的 Go 并发任务处理库。

## 特性

- **缓冲队列** - 默认 1000 大小的缓冲任务队列
- **任务优先级** - 支持 HIGH / NORMAL / LOW 三种优先级
- **饱和策略** - Abort / DropOldest / Block / CallerRuns 四种策略
- **任务超时** - 支持单个任务设置超时时间
- **优雅关闭** - 支持等待队列中任务完成
- **动态扩缩容** - 运行时调整 worker 数量
- **指标统计** - 实时监控任务提交、完成、失败数量
- **Panic 处理** - 捕获任务执行中的 panic，防止程序崩溃

## 安装

```bash
go get github.com/liliangshan/workpool
```

## 快速开始

### 简单用法

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/liliangshan/workpool/workpool"
)

func main() {
    // 创建包含 5 个 worker 的池
    pool := workpool.New(workpool.SimpleWithWorkerCount(5))
    pool.Start()

    // 提交任务
    for i := 0; i < 10; i++ {
        i := i
        pool.Submit(func(ctx context.Context) {
            fmt.Printf("Task %d completed\n", i)
            time.Sleep(100 * time.Millisecond)
        })
    }

    pool.Stop()
}
```

### 生产级用法

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/liliangshan/workpool/workpool"
)

func main() {
    // 创建配置
    config := &workpool.Config{
        WorkerCount: 4,
        QueueSize:  1000,
    }

    // 创建生产级池
    pool := workpool.NewPool(config)
    pool.Start()

    // 提交带选项的任务
    task := workpool.NewTaskWithOptions(
        func(ctx context.Context) {
            fmt.Println("Task executed")
        },
        workpool.TaskWithID("my-task-1"),
        workpool.TaskWithPriority(workpool.PriorityHigh),
        workpool.TaskWithTimeout(5 * time.Second),
    )

    pool.Submit(task)

    // 获取指标
    metrics := pool.Metrics()
    fmt.Printf("Completed: %d, Failed: %d\n",
        metrics.CompletedTasks, metrics.FailedTasks)

    pool.Stop()
}
```

## API 文档

### 简单 API (SimplePool)

适用于简单场景的轻量级 API。

| 函数 | 描述 |
|------|------|
| `New(opts...)` | 创建新的简单池 |
| `SimpleWithWorkerCount(n)` | 设置 worker 数量 |
| `pool.Start()` | 启动池 |
| `pool.Submit(task)` | 提交任务 |
| `pool.Stop()` | 停止池 |
| `pool.SubmitWithTimeout(task, timeout)` | 带超时的任务提交 |
| `pool.GracefulShutdown(timeout)` | 优雅关闭 |

### 生产 API (Pool)

功能完整的生产级 API。

| 函数 | 描述 |
|------|------|
| `NewPool(config)` | 创建生产级池 |
| `pool.Start()` | 启动池 |
| `pool.Stop()` | 停止池 |
| `pool.Submit(task)` | 提交任务 |
| `pool.SubmitWait(task)` | 提交并等待完成 |
| `pool.Resize(n)` | 动态调整 worker 数量 |
| `pool.GracefulShutdown(timeout)` | 优雅关闭 |
| `pool.Metrics()` | 获取指标快照 |
| `pool.SetSaturationStrategy(strategy)` | 设置饱和策略 |

### 任务选项

| 选项 | 描述 |
|------|------|
| `TaskWithID(id)` | 设置任务 ID |
| `TaskWithPriority(p)` | 设置优先级 |
| `TaskWithTimeout(d)` | 设置超时时间 |

### 优先级

```go
workpool.PriorityHigh    // 高优先级
workpool.PriorityNormal // 普通优先级
workpool.PriorityLow    // 低优先级
```

### 饱和策略

```go
workpool.Abort      // 队列满时返回错误
workpool.DropOldest // 丢弃最旧的任务
workpool.Block     // 阻塞直到有空间 (默认)
workpool.CallerRuns // 在调用者 goroutine 中执行
```

### 配置项

```go
type Config struct {
    WorkerCount int           // worker 数量，默认 CPU 核数
    QueueSize   int           // 任务队列大小，默认 1000
    TaskTimeout time.Duration // 任务超时时间
    PanicHandler func(interface{}) // panic 处理函数
    Logger      Logger        // 日志接口
}
```

### 指标统计

```go
type MetricsSnapshot struct {
    SubmittedTasks  int64           // 已提交任务数
    CompletedTasks int64           // 已完成任务数
    FailedTasks    int64           // 失败任务数
    ActiveWorkers  int64           // 活跃 worker 数
    IdleWorkers    int64           // 空闲 worker 数
    QueueSize      int64           // 当前队列大小
    AvgTaskWaitTime time.Duration  // 平均等待时间
    AvgTaskExecTime time.Duration  // 平均执行时间
}
```

### Logger 接口

```go
type Logger interface {
    Debug(msg string, keysAndValues ...interface{})
    Info(msg string, keysAndValues ...interface{})
    Warn(msg string, keysAndValues ...interface{})
    Error(msg string, keysAndValues ...interface{})
}
```

## 项目结构

```
workpool/
├── workpool.go       # 核心入口，简单 API
├── config.go         # 配置选项
├── task.go           # 任务定义和优先级队列
├── metrics.go        # 指标统计
├── pool.go           # 生产级池实现
├── worker.go         # worker 实现
└── examples/
    └── main.go       # 示例代码
```

## 运行示例

```bash
go run examples/main.go
```

## 性能考虑

1. **Worker 数量**: 建议设置为 CPU 核数，对于 IO 密集型任务可适当增加
2. **队列大小**: 根据任务提交速率和处理速率调整
3. **任务超时**: 合理设置超时时间，避免任务永久阻塞
4. **优雅关闭**: 生产环境建议使用优雅关闭，确保任务完成

## 许可证

Apache-2.0 License
