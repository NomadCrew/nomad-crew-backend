package services

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerPool_SubmitAndExecute(t *testing.T) {
	cfg := config.WorkerPoolConfig{
		MaxWorkers:             2,
		QueueSize:              10,
		ShutdownTimeoutSeconds: 5,
	}

	pool := NewWorkerPool(cfg)
	pool.Start()
	defer pool.Shutdown(context.Background())

	var executed int32
	done := make(chan struct{})

	submitted := pool.Submit(Job{
		Name: "test-job",
		Execute: func(ctx context.Context) error {
			atomic.AddInt32(&executed, 1)
			close(done)
			return nil
		},
	})

	require.True(t, submitted, "Job should be accepted")

	select {
	case <-done:
		// Job completed
	case <-time.After(2 * time.Second):
		t.Fatal("Job did not execute within timeout")
	}

	assert.Equal(t, int32(1), atomic.LoadInt32(&executed))
}

func TestWorkerPool_BoundedConcurrency(t *testing.T) {
	cfg := config.WorkerPoolConfig{
		MaxWorkers:             2,
		QueueSize:              100,
		ShutdownTimeoutSeconds: 5,
	}

	pool := NewWorkerPool(cfg)
	pool.Start()
	defer pool.Shutdown(context.Background())

	var maxConcurrent int32
	var currentConcurrent int32
	var mu sync.Mutex

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		pool.Submit(Job{
			Name: "concurrent-job",
			Execute: func(ctx context.Context) error {
				defer wg.Done()

				current := atomic.AddInt32(&currentConcurrent, 1)
				defer atomic.AddInt32(&currentConcurrent, -1)

				mu.Lock()
				if current > maxConcurrent {
					maxConcurrent = current
				}
				mu.Unlock()

				time.Sleep(50 * time.Millisecond)
				return nil
			},
		})
	}

	wg.Wait()

	assert.LessOrEqual(t, maxConcurrent, int32(2), "Should never exceed 2 concurrent workers")
}

func TestWorkerPool_QueueFull(t *testing.T) {
	cfg := config.WorkerPoolConfig{
		MaxWorkers:             1,
		QueueSize:              2,
		ShutdownTimeoutSeconds: 5,
	}

	pool := NewWorkerPool(cfg)
	pool.Start()
	defer pool.Shutdown(context.Background())

	// Block the worker
	blocker := make(chan struct{})
	pool.Submit(Job{
		Name: "blocker",
		Execute: func(ctx context.Context) error {
			<-blocker
			return nil
		},
	})

	// Fill the queue
	time.Sleep(10 * time.Millisecond) // Let worker pick up blocker
	pool.Submit(Job{Name: "queued-1", Execute: func(ctx context.Context) error { return nil }})
	pool.Submit(Job{Name: "queued-2", Execute: func(ctx context.Context) error { return nil }})

	// This should be dropped
	dropped := !pool.Submit(Job{Name: "overflow", Execute: func(ctx context.Context) error { return nil }})
	assert.True(t, dropped, "Job should be dropped when queue is full")

	close(blocker)
}

func TestWorkerPool_GracefulShutdown(t *testing.T) {
	cfg := config.WorkerPoolConfig{
		MaxWorkers:             2,
		QueueSize:              10,
		ShutdownTimeoutSeconds: 5,
	}

	pool := NewWorkerPool(cfg)
	pool.Start()

	var completed int32

	// Submit a slow job
	pool.Submit(Job{
		Name: "slow-job",
		Execute: func(ctx context.Context) error {
			time.Sleep(100 * time.Millisecond)
			atomic.AddInt32(&completed, 1)
			return nil
		},
	})

	// Give time for job to be picked up
	time.Sleep(10 * time.Millisecond)

	// Shutdown should wait for the job
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := pool.Shutdown(ctx)
	require.NoError(t, err)

	assert.Equal(t, int32(1), atomic.LoadInt32(&completed), "Job should complete during shutdown")
}

func TestWorkerPool_ShutdownTimeout(t *testing.T) {
	cfg := config.WorkerPoolConfig{
		MaxWorkers:             1,
		QueueSize:              10,
		ShutdownTimeoutSeconds: 1,
	}

	pool := NewWorkerPool(cfg)
	pool.Start()

	// Use a separate channel to control job completion - job ignores context
	jobDone := make(chan struct{})
	defer close(jobDone) // cleanup

	// Submit a job that ignores context and takes longer than shutdown timeout
	pool.Submit(Job{
		Name: "very-slow-job",
		Execute: func(ctx context.Context) error {
			// Intentionally ignore ctx.Done() to simulate uncooperative job
			select {
			case <-jobDone:
				return nil
			case <-time.After(10 * time.Second):
				return nil
			}
		},
	})

	// Give time for job to be picked up
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := pool.Shutdown(ctx)
	assert.Error(t, err, "Shutdown should timeout")
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestWorkerPool_DoubleStart(t *testing.T) {
	cfg := config.WorkerPoolConfig{
		MaxWorkers:             2,
		QueueSize:              10,
		ShutdownTimeoutSeconds: 5,
	}

	pool := NewWorkerPool(cfg)
	pool.Start()
	pool.Start() // Should be idempotent
	defer pool.Shutdown(context.Background())

	assert.True(t, pool.IsRunning())
}

func TestWorkerPool_JobError(t *testing.T) {
	cfg := config.WorkerPoolConfig{
		MaxWorkers:             1,
		QueueSize:              10,
		ShutdownTimeoutSeconds: 5,
	}

	pool := NewWorkerPool(cfg)
	pool.Start()
	defer pool.Shutdown(context.Background())

	var executed int32

	// First job errors
	pool.Submit(Job{
		Name: "error-job",
		Execute: func(ctx context.Context) error {
			atomic.AddInt32(&executed, 1)
			return assert.AnError
		},
	})

	// Second job should still run
	done := make(chan struct{})
	pool.Submit(Job{
		Name: "success-job",
		Execute: func(ctx context.Context) error {
			atomic.AddInt32(&executed, 1)
			close(done)
			return nil
		},
	})

	select {
	case <-done:
		// Good
	case <-time.After(2 * time.Second):
		t.Fatal("Second job did not execute")
	}

	assert.Equal(t, int32(2), atomic.LoadInt32(&executed), "Both jobs should execute")
}
