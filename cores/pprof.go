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

	"github.com/miebyte/goutils/logging"
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

	if cs.listenAddr == "" {
		logging.Warnf("Cores server not start by cores.Start(). Pprof can not be enabled")
		return
	}

	cs.httpMux.HandleFunc(pprofUrl, pprof.Index)
	cs.httpMux.HandleFunc(pprofUrl+"cmdline", pprof.Cmdline)
	cs.httpMux.HandleFunc(pprofUrl+"profile", pprof.Profile)
	cs.httpMux.HandleFunc(pprofUrl+"symbol", pprof.Symbol)
	cs.httpMux.HandleFunc(pprofUrl+"trace", pprof.Trace)
	cs.httpMux.Handle(pprofUrl+"goroutine", pprof.Handler("goroutine"))
	cs.httpMux.Handle(pprofUrl+"heap", pprof.Handler("heap"))
	cs.httpMux.Handle(pprofUrl+"threadcreate", pprof.Handler("threadcreate"))
	cs.httpMux.Handle(pprofUrl+"block", pprof.Handler("block"))

	_, port, _ := net.SplitHostPort(cs.listenAddr)
	target := fmt.Sprintf("localhost:%s", port)

	logging.Debugf("Pprof enabled. URL=%s", fmt.Sprintf("http://%s%s", target, pprofUrl))

}
