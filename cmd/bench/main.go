package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"time"

	"github.com/tengolang/tengo/v3"
	"github.com/tengolang/tengo/v3/parser"
)

type benchmark struct {
	name   string
	src    func(n int) string
	native func(n int) int
}

var benchmarks = []benchmark{
	{
		name: "fib",
		src: func(n int) string {
			return fmt.Sprintf(`
fib := func(x) {
	if x == 0 { return 0 } else if x == 1 { return 1 }
	return fib(x-1) + fib(x-2)
}
out = fib(%d)`, n)
		},
		native: func(n int) int { return fib(n) },
	},
	{
		name: "fibt1",
		src: func(n int) string {
			return fmt.Sprintf(`
fib := func(x, s) {
	if x == 0 { return 0 + s } else if x == 1 { return 1 + s }
	return fib(x-1, fib(x-2, s))
}
out = fib(%d, 0)`, n)
		},
		native: func(n int) int { return fibTC1(n, 0) },
	},
	{
		name: "fibt2",
		src: func(n int) string {
			return fmt.Sprintf(`
fib := func(x, a, b) {
	if x == 0 { return a } else if x == 1 { return b }
	return fib(x-1, b, a+b)
}
out = fib(%d, 0, 1)`, n)
		},
		native: func(n int) int { return fibTC2(n, 0, 1) },
	},
}

func main() {
	n := flag.Int("n", 35, "fibonacci input")
	count := flag.Int("count", 3, "number of runs (minimum is reported)")
	timeout := flag.Duration("timeout", 30*time.Second, "per-run timeout")
	flag.Parse()

	for _, b := range benchmarks {
		run(b, *n, *count, *timeout)
	}
}

func run(b benchmark, n, count int, timeout time.Duration) {
	goMin := time.Duration(math.MaxInt64)
	for i := 0; i < count; i++ {
		start := time.Now()
		b.native(n)
		if d := time.Since(start); d < goMin {
			goMin = d
		}
	}
	expected := b.native(n)

	src := []byte(b.src(n))

	parseMin := time.Duration(math.MaxInt64)
	compileMin := time.Duration(math.MaxInt64)
	vmMin := time.Duration(math.MaxInt64)

	for i := 0; i < count; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		pt, ct, vt, result, err := runBench(ctx, src)
		cancel()
		if err != nil {
			panic(fmt.Errorf("%s: %w", b.name, err))
		}
		if got := int(result.(tengo.Int).Value); got != expected {
			panic(fmt.Errorf("%s: wrong result %d != %d", b.name, got, expected))
		}
		if pt < parseMin {
			parseMin = pt
		}
		if ct < compileMin {
			compileMin = ct
		}
		if vt < vmMin {
			vmMin = vt
		}
	}

	ratio := float64(vmMin) / float64(goMin)
	fmt.Printf("%-8s  n=%-3d  result=%-10d  go=%s  parse=%s  compile=%s  vm=%s  ratio=%.1fx\n",
		b.name, n, expected, goMin, parseMin, compileMin, vmMin, ratio)
}

func fib(n int) int {
	if n <= 1 {
		return n
	}
	return fib(n-1) + fib(n-2)
}

func fibTC1(n, s int) int {
	if n == 0 {
		return s
	} else if n == 1 {
		return 1 + s
	}
	return fibTC1(n-1, fibTC1(n-2, s))
}

func fibTC2(n, a, b int) int {
	if n == 0 {
		return a
	} else if n == 1 {
		return b
	}
	return fibTC2(n-1, b, a+b)
}

func runBench(ctx context.Context, src []byte) (parseTime, compileTime, runTime time.Duration, result tengo.Object, err error) {
	fileSet := parser.NewFileSet()
	inputFile := fileSet.AddFile("bench", -1, len(src))

	start := time.Now()
	p := parser.NewParser(inputFile, src, nil)
	file, ferr := p.ParseFile()
	parseTime = time.Since(start)
	if ferr != nil {
		err = ferr
		return
	}

	symTable := tengo.NewSymbolTable()
	symTable.Define("out")

	start = time.Now()
	c := tengo.NewCompiler(file.InputFile, symTable, nil, nil, nil)
	if cerr := c.Compile(file); cerr != nil {
		compileTime = time.Since(start)
		err = cerr
		return
	}
	bytecode := c.Bytecode()
	bytecode.RemoveDuplicates()
	compileTime = time.Since(start)

	globals := make([]tengo.Object, tengo.GlobalsSize)
	start = time.Now()
	v := tengo.NewVM(bytecode, globals, -1)
	if rerr := v.RunContext(ctx); rerr != nil {
		runTime = time.Since(start)
		err = rerr
		return
	}
	runTime = time.Since(start)
	result = globals[0]
	return
}
