package macro

import (
	"math/rand"
	"time"
)

// Context provides execution state for running macros.
type Context struct {
	vars   map[string]string
	labels map[string]int
	pc     int
	rand   *rand.Rand
}

// NewContext creates a new execution context.
func NewContext() *Context {
	return &Context{
		vars:   make(map[string]string),
		labels: make(map[string]int),
		rand:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Var returns a variable value.
func (c *Context) Var(name string) string { return c.vars[name] }

// Execute runs the given commands.
func (c *Context) Execute(cmds []Command) error {
	c.pc = 0
	c.labels = make(map[string]int)
	// first pass: register labels
	for i, cmd := range cmds {
		if l, ok := cmd.(Label); ok {
			c.labels[l.Name] = i
		}
	}
	for c.pc < len(cmds) {
		cmd := cmds[c.pc]
		c.pc++
		if err := cmd.Exec(c); err != nil {
			return err
		}
	}
	return nil
}

// sleep waits the given duration in milliseconds. Separated for tests.
func (c *Context) sleep(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}
