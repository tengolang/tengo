package tengo_test

import (
	"fmt"
	"testing"

	tengo "github.com/ganehag/tengo/v3"
	"github.com/ganehag/tengo/v3/parser"
)

func benchScript(b *testing.B, src string) {
	b.Helper()
	fileSet := parser.NewFileSet()
	srcFile := fileSet.AddFile("bench", -1, len(src))
	p := parser.NewParser(srcFile, []byte(src), nil)
	file, err := p.ParseFile()
	if err != nil {
		b.Fatal(err)
	}

	symTable := tengo.NewSymbolTable()
	symTable.Define("out")
	c := tengo.NewCompiler(srcFile, symTable, nil, nil, nil)
	if err := c.Compile(file); err != nil {
		b.Fatal(err)
	}
	bytecode := c.Bytecode()
	bytecode.RemoveDuplicates()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		globals := make([]tengo.Object, tengo.GlobalsSize)
		v := tengo.NewVM(bytecode, globals, -1)
		if err := v.Run(); err != nil {
			b.Fatal(err)
		}
	}
}

func benchScriptCompile(b *testing.B, src string) {
	b.Helper()
	fileSet := parser.NewFileSet()
	srcFile := fileSet.AddFile("bench", -1, len(src))
	p := parser.NewParser(srcFile, []byte(src), nil)
	file, err := p.ParseFile()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		symTable := tengo.NewSymbolTable()
		symTable.Define("out")
		c := tengo.NewCompiler(srcFile, symTable, nil, nil, nil)
		if err := c.Compile(file); err != nil {
			b.Fatal(err)
		}
		bc := c.Bytecode()
		bc.RemoveDuplicates()
	}
}

const srcFibNaive = `
fib := func(x) {
	if x == 0 { return 0 }
	if x == 1 { return 1 }
	return fib(x-1) + fib(x-2)
}
out = fib(25)
`

func BenchmarkFibNaive(b *testing.B)        { benchScript(b, srcFibNaive) }
func BenchmarkFibNaiveCompile(b *testing.B) { benchScriptCompile(b, srcFibNaive) }

const srcFibTC = `
fib := func(x, a, b) {
	if x == 0 { return a }
	if x == 1 { return b }
	return fib(x-1, b, a+b)
}
out = fib(35, 0, 1)
`

func BenchmarkFibTailCall(b *testing.B)        { benchScript(b, srcFibTC) }
func BenchmarkFibTailCallCompile(b *testing.B) { benchScriptCompile(b, srcFibTC) }

const srcArithLoop = `
s := 0
for i := 0; i < 10000; i++ {
	s += i * i
}
out = s
`

func BenchmarkArithLoop(b *testing.B)        { benchScript(b, srcArithLoop) }
func BenchmarkArithLoopCompile(b *testing.B) { benchScriptCompile(b, srcArithLoop) }

const srcStringConcat = `
s := ""
for i := 0; i < 200; i++ {
	s += "x"
}
out = len(s)
`

func BenchmarkStringConcat(b *testing.B)        { benchScript(b, srcStringConcat) }
func BenchmarkStringConcatCompile(b *testing.B) { benchScriptCompile(b, srcStringConcat) }

const srcMapOps = `
m := {}
for i := 0; i < 500; i++ {
	m[string(i)] = i * 2
}
out = len(m)
`

func BenchmarkMapOps(b *testing.B)        { benchScript(b, srcMapOps) }
func BenchmarkMapOpsCompile(b *testing.B) { benchScriptCompile(b, srcMapOps) }

const srcArrayOps = `
a := []
for i := 0; i < 1000; i++ {
	a = append(a, i)
}
out = len(a)
`

func BenchmarkArrayOps(b *testing.B)        { benchScript(b, srcArrayOps) }
func BenchmarkArrayOpsCompile(b *testing.B) { benchScriptCompile(b, srcArrayOps) }

const srcMultiReturn = `
divmod := func(a, b) {
	return a/b, a%b
}
q := 0
r := 0
for i := 1; i <= 1000; i++ {
	q, r = divmod(i*7+3, i+1)
}
out = q + r
`

func BenchmarkMultiReturn(b *testing.B)        { benchScript(b, srcMultiReturn) }
func BenchmarkMultiReturnCompile(b *testing.B) { benchScriptCompile(b, srcMultiReturn) }

const srcClosures = `
make_adder := func(n) {
	return func(x) { return x + n }
}
add5 := make_adder(5)
s := 0
for i := 0; i < 1000; i++ {
	s += add5(i)
}
out = s
`

func BenchmarkClosures(b *testing.B)        { benchScript(b, srcClosures) }
func BenchmarkClosuresCompile(b *testing.B) { benchScriptCompile(b, srcClosures) }

var srcLarge = func() string {
	s := "s := 0\n"
	for i := 0; i < 50; i++ {
		s += fmt.Sprintf("s += %d\n", i)
	}
	s += "out = s\n"
	return s
}()

func BenchmarkLargeScriptCompile(b *testing.B) { benchScriptCompile(b, srcLarge) }
func BenchmarkLargeScriptRun(b *testing.B)     { benchScript(b, srcLarge) }
