package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/adityajoshi12/shedlock-go"
	"github.com/adityajoshi12/shedlock-go/providers/postgres"
)

func main() {
	// Connect to PostgreSQL
	db, err := sql.Open("postgres", "postgres://user:password@localhost:5432/shedlock?sslmode=disable")
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Create lock provider
	provider, err := postgres.NewPostgresLockProvider(postgres.Config{
		DB:        db,
		TableName: "shedlock",
	})
	if err != nil {
		log.Fatalf("Failed to create lock provider: %v", err)
	}

	// Create the shedlock table
	ctx := context.Background()
	if err := provider.CreateTable(ctx); err != nil {
		log.Fatalf("Failed to create table: %v", err)
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
