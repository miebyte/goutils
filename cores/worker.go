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

	"github.com/miebyte/goutils/logging"
	"github.com/pkg/errors"
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

func (c *CoresService) mountSimpleWorker(worker *simpleWorker) {
	fn := func(ctx context.Context) error {
		if err := worker.fn(ctx); err != nil && !errors.Is(err, context.Canceled) {
			logging.Errorc(ctx, "worker: %v run error: %v\n", worker.name, err)
			return errors.Wrapf(err, "simpleWorker: %v run failed", worker.name)
		}
		return nil
	}

	mf := mountFn{
		fn:      fn,
		maxWait: worker.maxWait,
		name:    worker.name,
	}

	c.mountFns = append(c.mountFns, mf)
}

func (c *CoresService) wrapWorker() {
	for _, worker := range c.workers {
		switch w := worker.(type) {
		case *simpleWorker:
			if w.name == "" {
				w.name = GetFuncName(w)
			}
			c.mountSimpleWorker(w)
		default:
			logging.Warnc(c.ctx, "Unknown worker type. worker: %v\n", GetFuncName(w))
		}
	}
}

func (c *CoresService) waitContext(ctx context.Context, maxWait time.Duration, fn func(context.Context) error) (err error) {
	defer func() {
		if errors.Is(err, context.Canceled) {
			err = nil
		}
	}()

	stop := make(chan error)

	go func() {
		stop <- fn(ctx)
	}()

	select {
	case err := <-stop:
		return err
	case <-ctx.Done():
		if c.waitAllDone {
			return waitAllDone(stop)
		} else {
			return waitUntilTimeout(ctx, stop, maxWait)
		}
	}
}

func waitAllDone(stop chan error) error {
	select {
	case err := <-stop:
		return errors.Wrap(err, "Stop")
	}
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

	logging.Warnc(ctx, "context Done, The worker will wait for a maximum of %v before being forced to stop", maxWait)

	t1.Reset(maxWait)
	select {
	case <-t1.C:
		logging.Warnc(ctx, "Force closing worker")
		return errors.Wrap(ctx.Err(), "Force close")
	case err := <-stop:
		return errors.Wrap(err, "Stop")
	}
}
