package scriptbench

import (
	"testing"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// compiledSum is the baseline compiled implementation.
func compiledSum() int {
	total := 0
	for i := 0; i < 1000; i++ {
		total += i
	}
	return total
}

// BenchmarkCompiledSum measures performance of compiled Go code.
func BenchmarkCompiledSum(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = compiledSum()
	}
}

// BenchmarkYaegiSum measures performance of the same code executed via the
// Yaegi interpreter.
func BenchmarkYaegiSum(b *testing.B) {
	const src = `
package main

func Sum() int {
    total := 0
    for i := 0; i < 1000; i++ {
        total += i
    }
    return total
}
`
	i := interp.New(interp.Options{})
	i.Use(stdlib.Symbols)
	if _, err := i.Eval(src); err != nil {
		b.Fatalf("eval: %v", err)
	}
	v, err := i.Eval("main.Sum")
	if err != nil {
		b.Fatalf("lookup: %v", err)
	}
	sum := v.Interface().(func() int)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sum()
	}
}
