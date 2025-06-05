// File:		localrepos.go
// Created by:	Hoven
// Created on:	2025-04-15
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package localrepos

import (
	"context"
	"iter"
	"sync"
	"time"

	"github.com/miebyte/goutils/logging"
)

type HashData interface {
	GetKey() string
}

type DataStore[T HashData] func(ctx context.Context) (iter.Seq[T], error)

type ReposKVEntry[T HashData] struct {
	Key   string
	Value T
}

type RepoOption struct {
	refreshInterval time.Duration
}

type LocalRepos[T HashData] struct {
	sync.RWMutex

	dataStore DataStore[T]
	data      map[string]T
	ticker    *time.Ticker

	*RepoOption
}

type ReposOptionsFunc func(*RepoOption)

var defaultRefreshInterval = 5 * time.Minute

func WithRefreshInterval(interval time.Duration) ReposOptionsFunc {
	return func(r *RepoOption) {
		r.refreshInterval = interval
	}
}

func NewLocalRepos[T HashData](dataStore DataStore[T], opts ...ReposOptionsFunc) *LocalRepos[T] {
	lp := &LocalRepos[T]{
		dataStore: dataStore,
	}

	o := new(RepoOption)
	o.refreshInterval = defaultRefreshInterval

	for _, opt := range opts {
		opt(o)
	}

	lp.RepoOption = o
	return lp
}

func (r *LocalRepos[T]) Start(ctx context.Context) {
	r.reloadEntries(ctx)
	r.ticker = time.NewTicker(r.refreshInterval)
	go func() {
		for range r.ticker.C {
			r.reloadEntries(ctx)
		}
	}()
}

func (r *LocalRepos[T]) reloadEntries(ctx context.Context) {
	iter, err := r.dataStore(ctx)
	if err != nil {
		logging.Errorc(ctx, "reloadEntries error: %v", err)
		return
	}

	newData := make(map[string]T)
	for item := range iter {
		newData[item.GetKey()] = item
	}

	r.Lock()
	r.data = newData
	r.Unlock()
}

func (r *LocalRepos[T]) AllValues() []T {
	r.RLock()
	defer r.RUnlock()
	var ret []T
	for _, v := range r.data {
		ret = append(ret, v)
	}
	return ret
}

func (r *LocalRepos[T]) AllKeys() []string {
	r.RLock()
	defer r.RUnlock()
	var ret []string
	for k := range r.data {
		ret = append(ret, k)
	}
	return ret
}

func (r *LocalRepos[T]) AllItems() []*ReposKVEntry[T] {
	r.RLock()
	defer r.RUnlock()
	var ret []*ReposKVEntry[T]
	for k, v := range r.data {
		ret = append(ret, &ReposKVEntry[T]{k, v})
	}
	return ret
}

func (r *LocalRepos[T]) Get(id string) T {
	r.RLock()
	defer r.RUnlock()
	return r.data[id]
}

func (r *LocalRepos[T]) Len() int {
	r.RLock()
	defer r.RUnlock()
	return len(r.data)
}

func (r *LocalRepos[T]) Close() error {
	r.ticker.Stop()
	return nil
}
