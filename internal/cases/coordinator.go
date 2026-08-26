package cases

import (
	"context"
	"sync"
	"time"

	"timber-pest-remediation-ledger/internal/domain"
)

type operation func(context.Context) (any, error)
type job struct {
	run    operation
	result chan jobResult
}
type jobResult struct {
	value any
	err   error
}

type caseWorker struct {
	ctx  context.Context
	jobs chan job
	stop chan struct{}
}

type Coordinator struct {
	mu          sync.Mutex
	workers     map[string]*caseWorker
	queueSize   int
	maxCases    int
	idleTimeout time.Duration
}

func NewCoordinator(queueSize, maxCases int, idle time.Duration) *Coordinator {
	if queueSize < 1 {
		queueSize = 16
	}
	if maxCases < 1 {
		maxCases = 1024
	}
	if idle <= 0 {
		idle = 5 * time.Minute
	}
	return &Coordinator{workers: make(map[string]*caseWorker), queueSize: queueSize, maxCases: maxCases, idleTimeout: idle}
}

func (c *Coordinator) Do(ctx context.Context, caseID string, run operation) (any, error) {
	worker, err := c.worker(ctx, caseID)
	if err != nil {
		return nil, err
	}
	result := make(chan jobResult, 1)
	item := job{run: run, result: result}
	select {
	case worker.jobs <- item:
	default:
		return nil, domain.Conflict("command_queue_full", "案件命令队列已满，请稍后重试")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case output := <-result:
		return output.value, output.err
	}
}

func (c *Coordinator) worker(ctx context.Context, caseID string) (*caseWorker, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if existing := c.workers[caseID]; existing != nil {
		return existing, nil
	}
	if len(c.workers) >= c.maxCases {
		return nil, domain.Conflict("coordinator_capacity", "活跃案件协调器已达到上限")
	}
	workerCtx := context.WithoutCancel(ctx)
	if ctx.Err() != nil {
		workerCtx = ctx
	}
	w := &caseWorker{ctx: workerCtx, jobs: make(chan job, c.queueSize), stop: make(chan struct{})}
	c.workers[caseID] = w
	go c.serve(caseID, w)
	return w, nil
}

func (c *Coordinator) serve(caseID string, worker *caseWorker) {
	timer := time.NewTimer(c.idleTimeout)
	defer timer.Stop()
	for {
		select {
		case item := <-worker.jobs:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			if worker.ctx.Err() != nil {
				item.result <- jobResult{err: worker.ctx.Err()}
			} else {
				value, err := item.run(worker.ctx)
				item.result <- jobResult{value: value, err: err}
			}
			timer.Reset(c.idleTimeout)
		case <-timer.C:
			c.mu.Lock()
			if len(worker.jobs) == 0 && c.workers[caseID] == worker {
				delete(c.workers, caseID)
				c.mu.Unlock()
				return
			}
			c.mu.Unlock()
			timer.Reset(c.idleTimeout)
		case <-worker.stop:
			return
		}
	}
}

func (c *Coordinator) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, worker := range c.workers {
		close(worker.stop)
		delete(c.workers, id)
	}
}
