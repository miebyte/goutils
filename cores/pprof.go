// File:		pprof.go
// Created by:	Hoven
// Created on:	2025-04-17
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package cores

import (
	"fmt"
	"net"
	"net/http/pprof"

	"github.com/miebyte/goutils/internal/innerlog"
)

const (
	pprofUrl = "/debug/pprof/"
)

func WithPprof() ServiceOption {
	return func(cs *CoresService) {
		cs.usePprof = true
	}
}

func (cs *CoresService) setupPprof() {
	if !cs.usePprof {
		return
	}

	if cs.internalHTTP.listener == nil {
		innerlog.Logger.Warnf("Cores server not start by cores.Start(). Pprof can not be enabled")
		return
	}

	cs.internalHTTP.mux.HandleFunc(pprofUrl, pprof.Index)
	cs.internalHTTP.mux.HandleFunc(pprofUrl+"cmdline", pprof.Cmdline)
	cs.internalHTTP.mux.HandleFunc(pprofUrl+"profile", pprof.Profile)
	cs.internalHTTP.mux.HandleFunc(pprofUrl+"symbol", pprof.Symbol)
	cs.internalHTTP.mux.HandleFunc(pprofUrl+"trace", pprof.Trace)
	cs.internalHTTP.mux.Handle(pprofUrl+"goroutine", pprof.Handler("goroutine"))
	cs.internalHTTP.mux.Handle(pprofUrl+"goroutineleak", pprof.Handler("goroutineleak"))
	cs.internalHTTP.mux.Handle(pprofUrl+"heap", pprof.Handler("heap"))
	cs.internalHTTP.mux.Handle(pprofUrl+"threadcreate", pprof.Handler("threadcreate"))
	cs.internalHTTP.mux.Handle(pprofUrl+"block", pprof.Handler("block"))

	_, port, _ := net.SplitHostPort(cs.internalHTTP.listener.Addr().String())
	target := fmt.Sprintf("localhost:%s", port)

	innerlog.Logger.Debugf("Pprof enabled. URL=%s", fmt.Sprintf("http://%s%s", target, pprofUrl))
}
