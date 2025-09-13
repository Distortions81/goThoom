package macro

import "testing"

func TestReplaceMacro(t *testing.T) {
	m, err := Parse("replace hi => hello {aIgnoreCase}")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	reg := NewRegistry()
	reg.Register(m)
	got := reg.Expand("HI there")
	if got != "hello there" {
		t.Fatalf("expand got %q", got)
	}
}

func TestFunctionExecution(t *testing.T) {
	src := `func greet {
set name world
label start
if name world done
goto start
label done
set greeting hello
}`
	m, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	reg := NewRegistry()
	reg.Register(m)
	ctx := NewContext()
	if err := reg.Invoke("greet", ctx); err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if ctx.Var("greeting") != "hello" {
		t.Fatalf("var not set, got %q", ctx.Var("greeting"))
	}
}
