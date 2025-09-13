package macro

import "sync"

// Registry stores macros and allows invocation.
type Registry struct {
	mu     sync.RWMutex
	macros map[string]*Macro
}

// NewRegistry creates a macro registry.
func NewRegistry() *Registry {
	return &Registry{macros: make(map[string]*Macro)}
}

// Register adds a macro to the registry.
func (r *Registry) Register(m *Macro) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m.Name == "" {
		m.Name = m.Trigger
	}
	r.macros[m.Name] = m
}

// Invoke runs a function or key macro by name.
func (r *Registry) Invoke(name string, ctx *Context) error {
	r.mu.RLock()
	m, ok := r.macros[name]
	r.mu.RUnlock()
	if !ok {
		return nil
	}
	switch m.Type {
	case Function:
		return ctx.Execute(m.Commands)
	case Key:
		return r.Invoke(m.Func, ctx)
	default:
		return nil
	}
}

// Expand applies all replacement/expression macros to the string.
func (r *Registry) Expand(s string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, m := range r.macros {
		if m.Type == Replace || m.Type == Expr {
			s = m.Expand(s)
		}
	}
	return s
}
