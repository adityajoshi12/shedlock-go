package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
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
	})
	if err != nil {
		log.Fatalf("Failed to create lock provider: %v", err)
	}

	// Create task executor
	executor := shedlock.NewDefaultLockingTaskExecutor(provider)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Define a scheduled task
	taskFunc := func(ctx context.Context) error {
		fmt.Printf("[%s] Executing task...\n", time.Now().Format("15:04:05"))
		time.Sleep(3 * time.Second)
		fmt.Printf("[%s] Task completed\n", time.Now().Format("15:04:05"))
		return nil
	}

	// Run task on a schedule
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	fmt.Println("Starting scheduler... (Press Ctrl+C to stop)")
	fmt.Println("Run multiple instances of this program to see distributed locking in action!")

	for {
		select {
		case <-ticker.C:
			lockConfig := shedlock.LockConfiguration{
				Name:           "scheduledTask",
				LockAtMostFor:  30 * time.Second,
				LockAtLeastFor: 0,
			}

			if err := executor.ExecuteWithLock(ctx, taskFunc, lockConfig); err != nil {
				log.Printf("Error executing task: %v", err)
			}

		case <-sigChan:
			fmt.Println("\nReceived shutdown signal. Exiting...")
			return
		}
	}
}

