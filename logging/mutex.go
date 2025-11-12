// File:		mutex.go
// Created by:	Hoven
// Created on:	2025-08-19
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package logging

import "sync"

type MutexWrap struct {
	lock     sync.Mutex
	disabled bool
}

func (mw *MutexWrap) Lock() {
	if !mw.disabled {
		mw.lock.Lock()
	}
}

func (mw *MutexWrap) Unlock() {
	if !mw.disabled {
		mw.lock.Unlock()
	}
}

func (mw *MutexWrap) Disable() {
	mw.disabled = true
}

type RWMutexWrap struct {
	lock     sync.RWMutex
	disabled bool
}

func (mw *RWMutexWrap) Lock() {
	if !mw.disabled {
		mw.lock.Lock()
	}
}

func (mw *RWMutexWrap) Unlock() {
	if !mw.disabled {
		mw.lock.Unlock()
	}
}

func (mw *RWMutexWrap) RLock() {
	if !mw.disabled {
		mw.lock.RLock()
	}
}

func (mw *RWMutexWrap) RUnlock() {
	if !mw.disabled {
		mw.lock.RUnlock()
	}
}

func (mw *RWMutexWrap) Disable() {
	mw.disabled = true
}
