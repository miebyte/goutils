// File:		discover.go
// Created by:	Hoven
// Created on:	2025-06-05
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package discover

import (
	"sync"
)

type Service struct {
	ServiceName string
	Address     string
	Tags        []string
}

type ServiceFinder interface {
	GetAddress(service string) string
	GetAllAddress(service string) []string
	GetAddressWithTag(service, tag string) string
	GetAllAddressWithTag(service, tag string) []string

	RegisterService(service, address string) error
	RegisterServiceWithTag(service, address, tag string) error
	RegisterServiceWithTags(service, address string, tags []string) error
	Close()
}

var (
	defaultServiceFinder ServiceFinder
	finderMutex          sync.RWMutex
)

func init() {
	defaultServiceFinder = NewDirectFinder()
}

func GetServiceFinder() ServiceFinder {
	finderMutex.RLock()
	defer finderMutex.RUnlock()
	return defaultServiceFinder
}

func SetFinder(finder ServiceFinder) {
	finderMutex.Lock()
	defer finderMutex.Unlock()
	defaultServiceFinder = finder
}

func GetAddress(srv string) string {
	return GetServiceFinder().GetAddress(srv)
}

func GetAddresses(srv string) []string {
	return GetServiceFinder().GetAllAddress(srv)
}

func GetAddressWithTag(srv, tag string) string {
	return GetServiceFinder().GetAddressWithTag(srv, tag)
}
