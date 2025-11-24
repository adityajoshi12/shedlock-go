package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/adityajoshi12/shedlock-go"
	"github.com/adityajoshi12/shedlock-go/providers/redis"
	goredis "github.com/redis/go-redis/v9"
)

func main() {
	// Connect to Redis
	client := goredis.NewClient(&goredis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	defer client.Close()

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Create lock provider
	provider, err := redis.NewRedisLockProvider(redis.Config{
		Client: client,
		Prefix: "shedlock:",
	})
	if err != nil {
		log.Fatalf("Failed to create lock provider: %v", err)
	}

	// Create task executor
	executor := shedlock.NewDefaultLockingTaskExecutor(provider)

	// Define your scheduled task
	task := func(ctx context.Context) error {
		fmt.Println("Executing scheduled task...")

		// Simulate some work
		time.Sleep(2 * time.Second)

		fmt.Println("Task completed successfully")
		return nil
	}

	// Execute task with lock
	lockConfig := shedlock.LockConfiguration{
		Name:           "myScheduledTask",
		LockAtMostFor:  10 * time.Minute, // Lock expires after 10 minutes
		LockAtLeastFor: 5 * time.Second,  // Keep lock for at least 5 seconds
	}

	// This would typically be called on a schedule (e.g., using a cron library)
	if err := executor.ExecuteWithLock(ctx, task, lockConfig); err != nil {
		log.Printf("Error executing task: %v", err)
	}

	fmt.Println("Example completed. In production, this would run on a schedule.")
}
