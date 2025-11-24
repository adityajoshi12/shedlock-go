# ShedLock-Go

[![CI](https://github.com/adityajoshi12/shedlock-go/actions/workflows/ci.yml/badge.svg)](https://github.com/adityajoshi12/shedlock-go/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/adityajoshi12/shedlock-go)](https://goreportcard.com/report/github.com/adityajoshi12/shedlock-go)
[![GoDoc](https://godoc.org/github.com/adityajoshi12/shedlock-go?status.svg)](https://godoc.org/github.com/adityajoshi12/shedlock-go)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

A distributed lock for your scheduled tasks in Go, inspired by [ShedLock](https://github.com/lukas-krecan/ShedLock).

ShedLock makes sure that your scheduled tasks are executed at most once at the same time. If a task is being executed on one node, it acquires a lock which prevents execution of the same task from another node (or thread). Please note, that **if one task is already being executed on one node, execution on other nodes does not wait, it is simply skipped**.

## Features

- 🔒 **Distributed Locking**: Ensure scheduled tasks run on only one node at a time
- 🚀 **Simple API**: Easy to use and integrate into your Go applications
- 🔌 **Multiple Storage Backends**: PostgreSQL, Redis (more coming soon)
- ⚡ **Lock Extension**: Extend lock duration for long-running tasks
- 🎯 **Configurable**: Set minimum and maximum lock durations
- 📊 **Logging Support**: Built-in logging interface for monitoring

## How It Works

ShedLock uses an external store (database, Redis, etc.) for coordination. When a scheduled task is about to execute:

1. It tries to acquire a lock with a unique name
2. If successful, the task executes
3. If the lock is already held by another node, execution is **skipped** (not queued)
4. The lock automatically expires after `lockAtMostFor` duration

This ensures that even if a node crashes, the lock will eventually be released and other nodes can acquire it.

## Installation

```bash
go get github.com/adityajoshi12/shedlock-go
```

For PostgreSQL support:
```bash
go get github.com/adityajoshi12/shedlock-go/providers/postgres
```

For Redis support:
```bash
go get github.com/adityajoshi12/shedlock-go/providers/redis
```

## Quick Start

### Using PostgreSQL

```go
package main

import (
    "context"
    "database/sql"
    "log"
    "time"

    "github.com/adityajoshi12/shedlock-go"
    "github.com/adityajoshi12/shedlock-go/providers/postgres"
    _ "github.com/lib/pq"
)

func main() {
    // Connect to PostgreSQL
    db, err := sql.Open("postgres", "postgres://user:password@localhost:5432/mydb?sslmode=disable")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Create lock provider
    provider, err := postgres.NewPostgresLockProvider(postgres.Config{
        DB:        db,
        TableName: "shedlock",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Create the shedlock table (run once)
    if err := provider.CreateTable(context.Background()); err != nil {
        log.Fatal(err)
    }

    // Create task executor
    executor := shedlock.NewDefaultLockingTaskExecutor(provider)

    // Define your task
    task := func(ctx context.Context) error {
        log.Println("Executing scheduled task...")
        // Your task logic here
        return nil
    }

    // Execute with lock
    err = executor.ExecuteWithLock(context.Background(), task, shedlock.LockConfiguration{
        Name:           "myScheduledTask",
        LockAtMostFor:  10 * time.Minute,
        LockAtLeastFor: 0,
    })
    if err != nil {
        log.Printf("Error: %v", err)
    }
}
```

### Using Redis

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/adityajoshi12/shedlock-go"
    "github.com/adityajoshi12/shedlock-go/providers/redis"
    goredis "github.com/redis/go-redis/v9"
)

func main() {
    // Connect to Redis
    client := goredis.NewClient(&goredis.Options{
        Addr: "localhost:6379",
    })
    defer client.Close()

    // Create lock provider
    provider, err := redis.NewRedisLockProvider(redis.Config{
        Client: client,
        Prefix: "shedlock:",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Create task executor
    executor := shedlock.NewDefaultLockingTaskExecutor(provider)

    // Define and execute your task
    task := func(ctx context.Context) error {
        log.Println("Executing scheduled task...")
        // Your task logic here
        return nil
    }

    err = executor.ExecuteWithLock(context.Background(), task, shedlock.LockConfiguration{
        Name:           "myScheduledTask",
        LockAtMostFor:  10 * time.Minute,
        LockAtLeastFor: 0,
    })
    if err != nil {
        log.Printf("Error: %v", err)
    }
}
```

## Configuration

### LockConfiguration

| Field | Type | Description |
|-------|------|-------------|
| `Name` | `string` | Unique name of the lock (required) |
| `LockAtMostFor` | `time.Duration` | Maximum duration the lock will be held. Prevents deadlock if node crashes (required) |
| `LockAtLeastFor` | `time.Duration` | Minimum duration the lock will be held. Prevents too frequent execution (optional) |

### Lock At Most For

The `LockAtMostFor` parameter specifies how long the lock will be held in case the executing node dies. This is a **safety mechanism** to prevent deadlocks. Set it to a value significantly longer than your task's expected execution time.

```go
LockAtMostFor: 10 * time.Minute  // Lock expires after 10 minutes
```

### Lock At Least For

The `LockAtLeastFor` parameter specifies the minimum time the lock should be held. This is useful for preventing a task from executing too frequently, even if it completes quickly.

```go
LockAtLeastFor: 5 * time.Second  // Lock held for at least 5 seconds
```

## Storage Providers

### PostgreSQL

The PostgreSQL provider stores locks in a dedicated table:

```sql
CREATE TABLE shedlock (
    name VARCHAR(64) PRIMARY KEY,
    lock_until TIMESTAMP NOT NULL,
    locked_at TIMESTAMP NOT NULL,
    locked_by VARCHAR(255) NOT NULL
);
```

**Configuration:**
```go
provider, err := postgres.NewPostgresLockProvider(postgres.Config{
    DB:        db,                // *sql.DB instance
    TableName: "shedlock",        // Optional, defaults to "shedlock"
})
```

### Redis

The Redis provider uses Redis keys with TTL for locking.

**Configuration:**
```go
provider, err := redis.NewRedisLockProvider(redis.Config{
    Client: client,               // *redis.Client instance
    Prefix: "shedlock:",          // Optional, defaults to "shedlock:"
})
```

## Integration with Schedulers

ShedLock works great with Go scheduling libraries. Here's an example with a simple ticker:

```go
ticker := time.NewTicker(1 * time.Minute)
defer ticker.Stop()

for range ticker.C {
    err := executor.ExecuteWithLock(ctx, task, lockConfig)
    if err != nil {
        log.Printf("Error: %v", err)
    }
}
```

You can also use it with popular cron libraries like:
- [robfig/cron](https://github.com/robfig/cron)
- [go-co-op/gocron](https://github.com/go-co-op/gocron)

## Advanced Usage

### Custom Logger

Implement the `Logger` interface for custom logging:

```go
type CustomLogger struct {
    // your logger implementation
}

func (l *CustomLogger) Debug(msg string, keysAndValues ...interface{}) { /* ... */ }
func (l *CustomLogger) Info(msg string, keysAndValues ...interface{}) { /* ... */ }
func (l *CustomLogger) Error(msg string, keysAndValues ...interface{}) { /* ... */ }

// Use custom logger
executor := shedlock.NewDefaultLockingTaskExecutorWithLogger(provider, &CustomLogger{})
```

### Lock Extension

For long-running tasks, you can extend the lock duration:

```go
// This requires direct access to the lock, which is managed internally
// Future versions may expose this functionality more directly
```

### Manual Lock Management

You can also manually manage locks without the executor:

```go
ctx := context.Background()
config := shedlock.LockConfiguration{
    Name:           "myLock",
    LockAtMostFor:  5 * time.Minute,
    LockAtLeastFor: 0,
}

lock, err := provider.Lock(ctx, config)
if err != nil {
    log.Fatal(err)
}

if lock == nil {
    log.Println("Lock is held by another process")
    return
}
defer lock.Unlock(ctx)

// Do your work here
log.Println("Lock acquired, executing task...")
```

## Examples

Check out the [examples](./examples) directory for complete working examples:

- [PostgreSQL Example](./examples/postgres/main.go)
- [Redis Example](./examples/redis/main.go)
- [Scheduler Example](./examples/scheduler/main.go) - Run multiple instances to see distributed locking in action!

## Best Practices

1. **Set appropriate timeouts**: `LockAtMostFor` should be significantly longer than your task's expected execution time
2. **Use unique lock names**: Each scheduled task should have a unique lock name
3. **Handle errors**: Always check and log errors from `ExecuteWithLock`
4. **Use `LockAtLeastFor`** for short tasks: Prevents too frequent execution
5. **Monitor your locks**: Use the built-in logging to monitor lock acquisition and release

## Caveats

Locks in ShedLock have an expiration time which leads to the following possible issues:

1. **If the task runs longer than `lockAtMostFor`**, the task can be executed more than once
2. **Clock skew**: If the clock difference between nodes is significant, it may affect lock behavior
3. **Network issues**: Network partitions or delays can affect lock acquisition

## Differences from Java ShedLock

This Go implementation maintains the core concepts of the original Java ShedLock while following Go idioms:

- Uses Go's `context.Context` for cancellation and timeouts
- Uses Go's `time.Duration` instead of Java's duration strings
- Follows Go error handling patterns
- Uses Go interfaces for extensibility

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

Apache License 2.0

## Acknowledgments

Inspired by the excellent [ShedLock](https://github.com/lukas-krecan/ShedLock) library for Java by Lukas Krecan.

## Roadmap

- [ ] MySQL provider
- [ ] MongoDB provider
- [ ] DynamoDB provider
- [ ] Metrics and monitoring support
- [ ] More comprehensive testing
- [ ] Benchmarking and performance optimization

