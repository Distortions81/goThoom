package macro

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

// Parse parses a macro definition from a string.
func Parse(src string) (*Macro, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil, fmt.Errorf("empty macro")
	}
	fields := strings.Fields(src)
	if len(fields) == 0 {
		return nil, fmt.Errorf("invalid macro")
	}
	switch strings.ToLower(fields[0]) {
	case "expr", "expression":
		return parseReplace(Expr, src)
	case "repl", "replace":
		return parseReplace(Replace, src)
	case "func", "function":
		return parseFunction(src)
	case "key":
		return parseKey(src)
	default:
		return nil, fmt.Errorf("unknown macro type %q", fields[0])
	}
}

func parseReplace(t Type, src string) (*Macro, error) {
	// format: type trigger => replacement {attr1,attr2}
	// split attributes
	attrs := Attribute(0)
	if i := strings.Index(src, "{"); i >= 0 {
		j := strings.LastIndex(src, "}")
		if j < 0 || j < i {
			return nil, fmt.Errorf("missing closing }")
		}
		parts := strings.Split(src[i+1:j], ",")
		for _, p := range parts {
			a, err := ParseAttribute(strings.TrimSpace(p))
			if err != nil {
				return nil, err
			}
			attrs |= a
		}
		src = strings.TrimSpace(src[:i])
	}
	seg := strings.Split(src, "=>")
	if len(seg) != 2 {
		return nil, fmt.Errorf("missing => in macro")
	}
	header := strings.TrimSpace(seg[0])
	parts := strings.Fields(header)
	if len(parts) < 2 {
		return nil, fmt.Errorf("missing trigger in macro")
	}
	trigger := strings.Join(parts[1:], " ")
	repl := strings.TrimSpace(seg[1])
	m := &Macro{Type: t, Trigger: trigger, Replacement: repl, Attr: attrs}
	if err := m.prepare(); err != nil {
		return nil, err
	}
	return m, nil
}

func parseFunction(src string) (*Macro, error) {
	// format: func name {\n commands \n}
	i := strings.Index(src, "{")
	j := strings.LastIndex(src, "}")
	if i < 0 || j < 0 || j < i {
		return nil, fmt.Errorf("function macro must have braces")
	}
	header := strings.TrimSpace(src[:i])
	fields := strings.Fields(header)
	if len(fields) < 2 {
		return nil, fmt.Errorf("function macro missing name")
	}
	name := fields[1]
	body := src[i+1 : j]
	cmds, err := parseCommands(body)
	if err != nil {
		return nil, err
	}
	return &Macro{Type: Function, Name: name, Commands: cmds}, nil
}

func parseKey(src string) (*Macro, error) {
	// format: key combo => func {attrs}
	attrs := Attribute(0)
	if i := strings.Index(src, "{"); i >= 0 {
		j := strings.LastIndex(src, "}")
		if j < 0 || j < i {
			return nil, fmt.Errorf("missing closing }")
		}
		parts := strings.Split(src[i+1:j], ",")
		for _, p := range parts {
			a, err := ParseAttribute(strings.TrimSpace(p))
			if err != nil {
				return nil, err
			}
			attrs |= a
		}
		src = strings.TrimSpace(src[:i])
	}
	seg := strings.Split(src, "=>")
	if len(seg) != 2 {
		return nil, fmt.Errorf("missing => in key macro")
	}
	left := strings.TrimSpace(seg[0])
	right := strings.TrimSpace(seg[1])
	fields := strings.Fields(left)
	if len(fields) < 2 {
		return nil, fmt.Errorf("key macro missing combo")
	}
	combo := fields[1]
	return &Macro{Type: Key, Trigger: combo, Func: right, Attr: attrs}, nil
}

func parseCommands(body string) ([]Command, error) {
	var cmds []Command
	scanner := bufio.NewScanner(strings.NewReader(body))
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		fields := strings.Fields(text)
		switch strings.ToLower(fields[0]) {
		case "pause":
			if len(fields) != 2 {
				return nil, fmt.Errorf("pause needs duration on line %d", line)
			}
			v, err := strconv.Atoi(fields[1])
			if err != nil {
				return nil, err
			}
			cmds = append(cmds, Pause{Duration: v})
		case "set":
			if len(fields) < 3 {
				return nil, fmt.Errorf("set needs name and value on line %d", line)
			}
			cmds = append(cmds, Set{Name: fields[1], Value: strings.Join(fields[2:], " ")})
		case "label":
			if len(fields) != 2 {
				return nil, fmt.Errorf("label needs name on line %d", line)
			}
			cmds = append(cmds, Label{Name: fields[1]})
		case "goto":
			if len(fields) != 2 {
				return nil, fmt.Errorf("goto needs label on line %d", line)
			}
			cmds = append(cmds, Goto{Label: fields[1]})
		case "if":
			// format: if var value label
			if len(fields) != 4 {
				return nil, fmt.Errorf("if needs var value label on line %d", line)
			}
			cmds = append(cmds, If{Var: fields[1], Value: fields[2], Label: fields[3]})
		case "random":
			if len(fields) < 2 {
				return nil, fmt.Errorf("random needs labels on line %d", line)
			}
			cmds = append(cmds, Random{Labels: fields[1:]})
		default:
			return nil, fmt.Errorf("unknown command %q on line %d", fields[0], line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return cmds, nil
}
