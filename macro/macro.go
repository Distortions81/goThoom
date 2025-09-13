package macro

import (
	"fmt"
	"regexp"
	"strings"
)

// Type represents the macro type.
type Type int

const (
	Expr     Type = iota // expression macro using regular expressions
	Replace              // simple replacement macro
	Function             // function macro executing commands
	Key                  // key macro binding a key combination to a function
)

// Attribute is a bitmask of macro attributes mirrored from MacroDefs_cl.h.
type Attribute uint32

const (
	AIgnoreCase Attribute = 1 << iota // aIgnoreCase - match case insensitive
	AAnyClick                         // aAnyClick - key macro triggers on any mouse button
	AKeyUp                            // aKeyUp - key macro triggers on key up
)

// Macro describes a single macro definition.
type Macro struct {
	Name        string
	Type        Type
	Trigger     string
	Replacement string
	Func        string // for key macros: function to invoke
	Attr        Attribute
	Commands    []Command // for function macros
	pattern     *regexp.Regexp
}

// prepare compiles pattern for expression or replacement macros.
func (m *Macro) prepare() error {
	if m.Type == Expr || m.Type == Replace {
		pat := m.Trigger
		if m.Type == Replace {
			pat = regexp.QuoteMeta(pat)
		}
		if m.Attr&AIgnoreCase != 0 {
			pat = "(?i)" + pat
		}
		re, err := regexp.Compile(pat)
		if err != nil {
			return err
		}
		m.pattern = re
	}
	return nil
}

// Expand applies the macro to the given input text.
func (m *Macro) Expand(s string) string {
	switch m.Type {
	case Replace:
		if m.Attr&AIgnoreCase != 0 {
			return m.pattern.ReplaceAllStringFunc(s, func(_ string) string { return m.Replacement })
		}
		return strings.ReplaceAll(s, m.Trigger, m.Replacement)
	case Expr:
		if m.pattern == nil {
			return s
		}
		return m.pattern.ReplaceAllString(s, m.Replacement)
	default:
		return s
	}
}

// ParseAttribute converts an attribute string to the bit flag.
func ParseAttribute(s string) (Attribute, error) {
	switch s {
	case "aIgnoreCase":
		return AIgnoreCase, nil
	case "aAnyClick":
		return AAnyClick, nil
	case "aKeyUp":
		return AKeyUp, nil
	case "":
		return 0, nil
	default:
		return 0, fmt.Errorf("unknown attribute %q", s)
	}
}

// Command represents a single executable macro command.
type Command interface {
	Exec(*Context) error
}

// Pause pauses execution for the specified milliseconds.
type Pause struct{ Duration int }

func (p Pause) Exec(ctx *Context) error {
	ctx.sleep(p.Duration)
	return nil
}

// Set assigns a variable in the context.
type Set struct{ Name, Value string }

func (s Set) Exec(ctx *Context) error {
	ctx.vars[s.Name] = s.Value
	return nil
}

// Label marks a location in the command list.
type Label struct{ Name string }

func (l Label) Exec(ctx *Context) error {
	ctx.labels[l.Name] = ctx.pc
	return nil
}

// Goto jumps to a label.
type Goto struct{ Label string }

func (g Goto) Exec(ctx *Context) error {
	idx, ok := ctx.labels[g.Label]
	if !ok {
		return fmt.Errorf("label %s not found", g.Label)
	}
	ctx.pc = idx
	return nil
}

// If compares a variable and jumps to a label if it matches.
type If struct{ Var, Value, Label string }

func (i If) Exec(ctx *Context) error {
	if ctx.vars[i.Var] == i.Value {
		return Goto{Label: i.Label}.Exec(ctx)
	}
	return nil
}

// Random chooses a label at random.
type Random struct{ Labels []string }

func (r Random) Exec(ctx *Context) error {
	if len(r.Labels) == 0 {
		return nil
	}
	n := ctx.rand.Intn(len(r.Labels))
	return Goto{Label: r.Labels[n]}.Exec(ctx)
}
