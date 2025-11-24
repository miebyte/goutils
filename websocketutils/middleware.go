package websocketutils

import "sync"

// middlewareChain 负责存储与执行一组中间件。
type middlewareChain struct {
	mu    sync.RWMutex
	chain []Middleware
}

func (m *middlewareChain) Add(mw Middleware) {
	if mw == nil {
		return
	}
	m.mu.Lock()
	m.chain = append(m.chain, mw)
	m.mu.Unlock()
}

func (m *middlewareChain) Run(conn Conn) error {
	m.mu.RLock()
	chain := append([]Middleware(nil), m.chain...)
	m.mu.RUnlock()
	for _, mw := range chain {
		if err := mw(conn); err != nil {
			return err
		}
	}
	return nil
}
