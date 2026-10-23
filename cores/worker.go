// File:		worker.go
// Created by:	Hoven
// Created on:	2025-04-02
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package cores

import (
	"context"
	"time"

	"sync/atomic"

	"github.com/miebyte/goutils/internal/innerlog"
	"github.com/miebyte/goutils/logging"
	"github.com/pkg/errors"
	"github.com/robfig/cron/v3"
)

var (
	defaultMaxWait = time.Second * 5
)

type WorkerFunc func(ctx context.Context) error

type Worker interface {
	Name() string
	Fn(ctx context.Context) error
}

type base struct {
	name    string
	maxWait time.Duration
	fn      WorkerFunc
}

type simpleWorker struct {
	*base
}

func (w *simpleWorker) Name() string {
	return w.name
}

func (w *simpleWorker) Fn(ctx context.Context) error {
	return w.fn(ctx)
}

func withSimpleWorker(name string, fn WorkerFunc, maxWaits ...time.Duration) ServiceOption {
	maxWait := defaultMaxWait
	if len(maxWaits) != 0 {
		maxWait = maxWaits[0]
	}
	return func(cs *CoresService) {
		cs.workers = append(cs.workers, &simpleWorker{
			base: &base{
				name:    name,
				maxWait: maxWait,
				fn:      fn,
			},
		})
	}
}

func WithWorker(fn WorkerFunc, maxWaits ...time.Duration) ServiceOption {
	return withSimpleWorker(logging.GetFuncName(fn), fn, maxWaits...)
}

func WithNameWorker(name string, fn WorkerFunc, maxWaits ...time.Duration) ServiceOption {
	return withSimpleWorker(name, fn, maxWaits...)
}

type cronWorker struct {
	*simpleWorker
	cronConf string
	schedule cron.Schedule
	jobId    cron.EntryID
	running  int32
	// whether the cron job should run exclusively (no overlap)
	exclusive bool
}

func (cw *cronWorker) Next(c *cron.Cron) time.Time {
	return c.Entry(cw.jobId).Next
}

type cronWorkerOptionFn func(*cronWorker)

// CronExclusive controls whether the cron worker runs exclusively (no overlap)
// when enabled. Default is enabled to keep backward compatibility.
func CronExclusive(enabled bool) cronWorkerOptionFn {
	return func(cw *cronWorker) {
		cw.exclusive = enabled
	}
}

func WithCronWorker(name string, cronConf string, fn WorkerFunc, opts ...cronWorkerOptionFn) ServiceOption {
	maxWait := defaultMaxWait

	schedule, err := cron.ParseStandard(cronConf)
	innerlog.Logger.PanicError(err)

	return func(cs *CoresService) {
		cw := &cronWorker{
			simpleWorker: &simpleWorker{
				base: &base{
					name:    name,
					maxWait: maxWait,
					fn:      fn,
				},
			},
			cronConf:  cronConf,
			schedule:  schedule,
			exclusive: false,
		}

		for _, opt := range opts {
			opt(cw)
		}

		cs.workers = append(cs.workers, cw)
	}
}

func (c *CoresService) mountSimpleWorker(worker *simpleWorker) {
	fn := func(ctx context.Context) error {
		if err := worker.fn(ctx); err != nil && !errors.Is(err, context.Canceled) {
			innerlog.Logger.Errorc(ctx, "worker: %v run error: %v\n", worker.name, err)
			return errors.Wrapf(err, "simpleWorker: %v run failed", worker.name)
		}
		return nil
	}

	mf := mountFn{
		fn:          fn,
		maxWait:     worker.maxWait,
		waitAllDone: c.waitAllDone,
		name:        worker.name,
	}

	c.mountFns = append(c.mountFns, mf)
}

func runCronJob(ctx context.Context, cron *cron.Cron, worker *cronWorker) cron.FuncJob {
	return func() {
		workerCtx := logging.With(ctx, "worker", worker.name)
		if worker.exclusive {
			if !atomic.CompareAndSwapInt32(&worker.running, 0, 1) {
				next := worker.Next(cron)
				innerlog.Logger.Warnc(workerCtx, "skip cron worker: %v is still running, next: %v", worker.name, next)
				return
			}
			defer atomic.StoreInt32(&worker.running, 0)
		}

		err := waitContext(workerCtx, worker.maxWait, false, worker.fn)
		if err != nil {
			innerlog.Logger.Errorc(workerCtx, "cron func run error: %v", err)
		}

		select {
		case <-workerCtx.Done():
			return
		default:
		}

		next := worker.Next(cron)
		innerlog.Logger.Infoc(workerCtx, "next cron worker: %v, next: %v", worker.name, next)
	}
}

func (c *CoresService) mountCronWorker(worker *cronWorker) {
	if c.cron == nil {
		c.cron = cron.New()
	}

	worker.jobId = c.cron.Schedule(worker.schedule, runCronJob(c.ctx, c.cron, worker))
	innerlog.Logger.Infoc(c.ctx, "mount cron worker(%s), schedule at: %v", worker.name, worker.schedule.Next(time.Now()))
}

func (c *CoresService) wrapWorker() {
	for _, worker := range c.workers {
		switch w := worker.(type) {
		case *simpleWorker:
			if w.name == "" {
				w.name = GetFuncName(w)
			}
			c.mountSimpleWorker(w)
		case *cronWorker:
			if w.name == "" {
				w.name = GetFuncName(w)
			}
			c.mountCronWorker(w)
		default:
			innerlog.Logger.Warnc(c.ctx, "Unknown worker type. worker: %v\n", GetFuncName(w))
		}
	}

	if c.cron != nil {
		c.mountSimpleWorker(&simpleWorker{
			base: &base{
				name:    "cron",
				maxWait: defaultMaxWait,
				fn: func(ctx context.Context) error {
					c.cron.Start()

					<-ctx.Done()
					stopWati := c.cron.Stop()
					innerlog.Logger.Infoc(ctx, "waiting cron worker stop. worker count: %v", len(c.cron.Entries()))
					<-stopWati.Done()
					return nil
				},
			},
		})
	}
}

func waitContext(ctx context.Context, maxWait time.Duration, isWaitDone bool, fn func(context.Context) error) (err error) {
	defer func() {
		if errors.Is(err, context.Canceled) {
			err = nil
		}
	}()

	stop := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				panic(r)
			}
		}()

		stop <- fn(ctx)
	}()

	select {
	case err := <-stop:
		return err
	case <-ctx.Done():
		if isWaitDone {
			return waitDone(stop)
		} else {
			return waitUntilTimeout(ctx, stop, maxWait)
		}
	}
}

func waitDone(stop chan error) error {
	err := <-stop
	return errors.Wrap(err, "Stop")
}

func waitUntilTimeout(ctx context.Context, stop chan error, maxWait time.Duration) error {
	if maxWait == 0 {
		return ctx.Err()
	}

	t1 := time.NewTicker(time.Second)
	defer t1.Stop()
	select {
	case err := <-stop:
		return errors.Wrap(err, "Stop")
	case <-t1.C:
	}

	innerlog.Logger.Warnc(ctx, "context Done, The worker will wait for a maximum of %v before being forced to stop", maxWait)

	t1.Reset(maxWait)
	select {
	case <-t1.C:
		innerlog.Logger.Warnc(ctx, "Force closing worker")
		return errors.Wrap(ctx.Err(), "Force close")
	case err := <-stop:
		return errors.Wrap(err, "Stop")
	}
}
