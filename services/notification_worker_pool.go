// Package services provides business logic implementations.
package services

import (
	"context"
	"sync"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// Job represents a unit of work for the worker pool.
type Job struct {
	// Name is a descriptive name for logging purposes
	Name string
	// Execute is the function that performs the work
	Execute func(ctx context.Context) error
}

// WorkerPool manages a bounded set of workers processing jobs from a queue.
// It provides graceful shutdown with configurable timeout and exposes
// Prometheus metrics for monitoring.
type WorkerPool struct {
	jobQueue chan Job
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	logger   *zap.SugaredLogger
	metrics  *workerPoolMetrics
	config   config.WorkerPoolConfig
	mu       sync.Mutex
	running  bool
}

// workerPoolMetrics holds Prometheus metrics for the worker pool.
type workerPoolMetrics struct {
	queueDepth    prometheus.Gauge
	activeWorkers prometheus.Gauge
	completedJobs prometheus.Counter
	droppedJobs   prometheus.Counter
	errorCount    prometheus.Counter
	jobDuration   prometheus.Histogram
}

// Singleton pattern for metrics (avoid double registration in tests).
var (
	wpMetricsInstance *workerPoolMetrics
	wpMetricsOnce     sync.Once
	wpDefaultRegistry = prometheus.DefaultRegisterer
)

// newWorkerPoolMetrics initializes and registers Prometheus metrics using singleton pattern.
func newWorkerPoolMetrics() *workerPoolMetrics {
	wpMetricsOnce.Do(func() {
		wpMetricsInstance = &workerPoolMetrics{
			queueDepth: promauto.With(wpDefaultRegistry).NewGauge(prometheus.GaugeOpts{
				Name: "notification_worker_pool_queue_depth",
				Help: "Current number of jobs waiting in queue",
			}),
			activeWorkers: promauto.With(wpDefaultRegistry).NewGauge(prometheus.GaugeOpts{
				Name: "notification_worker_pool_active_workers",
				Help: "Current number of workers processing jobs",
			}),
			completedJobs: promauto.With(wpDefaultRegistry).NewCounter(prometheus.CounterOpts{
				Name: "notification_worker_pool_completed_jobs_total",
				Help: "Total number of successfully completed jobs",
			}),
			droppedJobs: promauto.With(wpDefaultRegistry).NewCounter(prometheus.CounterOpts{
				Name: "notification_worker_pool_dropped_jobs_total",
				Help: "Total number of jobs dropped due to full queue",
			}),
			errorCount: promauto.With(wpDefaultRegistry).NewCounter(prometheus.CounterOpts{
				Name: "notification_worker_pool_errors_total",
				Help: "Total number of job execution errors",
			}),
			jobDuration: promauto.With(wpDefaultRegistry).NewHistogram(prometheus.HistogramOpts{
				Name:    "notification_worker_pool_job_duration_seconds",
				Help:    "Time taken to execute jobs",
				Buckets: []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10},
			}),
		}
	})
	return wpMetricsInstance
}

// resetWorkerPoolMetricsForTesting resets the metrics singleton for test isolation.
// This should only be called from tests.
func resetWorkerPoolMetricsForTesting() {
	reg := prometheus.NewRegistry()
	wpDefaultRegistry = reg
	wpMetricsInstance = nil
	wpMetricsOnce = sync.Once{}
}

// NewWorkerPool creates a new worker pool with the given configuration.
// The pool must be started with Start() before submitting jobs.
func NewWorkerPool(cfg config.WorkerPoolConfig) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		jobQueue: make(chan Job, cfg.QueueSize),
		ctx:      ctx,
		cancel:   cancel,
		logger:   logger.GetLogger().Named("worker-pool"),
		metrics:  newWorkerPoolMetrics(),
		config:   cfg,
	}
}

// Start launches the worker goroutines. Calling Start() multiple times is safe
// and will only start workers once.
func (wp *WorkerPool) Start() {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if wp.running {
		wp.logger.Warn("Worker pool already running")
		return
	}
	wp.running = true

	wp.logger.Infow("Starting worker pool",
		"maxWorkers", wp.config.MaxWorkers,
		"queueSize", wp.config.QueueSize)

	for i := 0; i < wp.config.MaxWorkers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// worker is the main loop for each worker goroutine.
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()
	wp.logger.Debugw("Worker started", "workerId", id)

	for {
		select {
		case <-wp.ctx.Done():
			wp.logger.Debugw("Worker stopping (context cancelled)", "workerId", id)
			return
		case job, ok := <-wp.jobQueue:
			if !ok {
				wp.logger.Debugw("Worker stopping (channel closed)", "workerId", id)
				return
			}
			wp.executeJob(id, job)
		}
	}
}

// executeJob runs a single job with metrics and error handling.
func (wp *WorkerPool) executeJob(workerID int, job Job) {
	wp.metrics.activeWorkers.Inc()
	wp.metrics.queueDepth.Dec()

	start := time.Now()

	// Create a context with timeout for the job (30 second default)
	jobCtx, cancel := context.WithTimeout(wp.ctx, 30*time.Second)
	defer cancel()

	if err := job.Execute(jobCtx); err != nil {
		wp.logger.Errorw("Job execution failed",
			"job", job.Name,
			"workerId", workerID,
			"error", err,
			"duration", time.Since(start))
		wp.metrics.errorCount.Inc()
	} else {
		wp.logger.Debugw("Job completed",
			"job", job.Name,
			"workerId", workerID,
			"duration", time.Since(start))
	}

	wp.metrics.jobDuration.Observe(time.Since(start).Seconds())
	wp.metrics.completedJobs.Inc()
	wp.metrics.activeWorkers.Dec()
}

// Submit adds a job to the queue. Returns true if the job was queued,
// false if the queue is full and the job was dropped.
// This method is non-blocking and safe to call from multiple goroutines.
func (wp *WorkerPool) Submit(job Job) bool {
	select {
	case wp.jobQueue <- job:
		wp.metrics.queueDepth.Inc()
		wp.logger.Debugw("Job submitted", "job", job.Name)
		return true
	default:
		wp.metrics.droppedJobs.Inc()
		wp.logger.Warnw("Job dropped - queue full",
			"job", job.Name,
			"queueSize", wp.config.QueueSize)
		return false
	}
}

// Shutdown gracefully stops the worker pool, waiting for in-flight jobs to complete.
// The provided context controls the maximum time to wait for workers to finish.
// Returns ctx.Err() if the context times out before all workers finish.
func (wp *WorkerPool) Shutdown(ctx context.Context) error {
	wp.mu.Lock()
	if !wp.running {
		wp.mu.Unlock()
		return nil
	}
	wp.running = false
	wp.mu.Unlock()

	wp.logger.Info("Initiating worker pool shutdown...")

	// Signal workers to stop accepting new jobs from context
	wp.cancel()

	// Close channel so workers exit their loops after draining
	close(wp.jobQueue)

	// Wait for workers with timeout
	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		wp.logger.Info("Worker pool shutdown complete - all workers finished")
		return nil
	case <-ctx.Done():
		wp.logger.Warn("Worker pool shutdown timed out - some workers may still be running")
		return ctx.Err()
	}
}

// QueueDepth returns the current number of jobs waiting in the queue.
func (wp *WorkerPool) QueueDepth() int {
	return len(wp.jobQueue)
}

// IsRunning returns whether the worker pool is currently running.
func (wp *WorkerPool) IsRunning() bool {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	return wp.running
}
