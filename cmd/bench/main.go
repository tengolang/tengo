package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/tengolang/tengo/v3"
	"github.com/tengolang/tengo/v3/parser"
)

// --- detailed mode ---

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

// --- markdown comparison table ---

const specialTengo = "tengo"
const specialGo = "go"

type langEntry struct {
	name    string
	link    string
	typ     string
	special string   // "tengo" or "go" for native runners; empty = external
	binary  string   // external binary name
	ext     string   // script file extension (fib.<ext>, fibtc.<ext>)
	flags   []string // extra flags inserted before the script path
}

var languages = []langEntry{
	{
		name:    "**Tengo**",
		link:    "https://github.com/tengolang/tengo",
		typ:     "Tengo (VM)",
		special: specialTengo,
	},
	{
		name: "go-lua", link: "https://github.com/Shopify/go-lua",
		typ: "Lua (VM)", binary: "go-lua", ext: "lua",
	},
	{
		name: "GopherLua", link: "https://github.com/yuin/gopher-lua",
		typ: "Lua (VM)", binary: "glua", ext: "lua",
	},
	{
		name: "goja", link: "https://github.com/dop251/goja",
		typ: "JavaScript (VM)", binary: "goja", ext: "js",
	},
	{
		name: "starlark-go", link: "https://github.com/google/starlark-go",
		typ: "Starlark (Interpreter)", binary: "starlark", ext: "star",
		flags: []string{"-recursion"},
	},
	{
		name: "gpython", link: "https://github.com/go-python/gpython",
		typ: "Python (Interpreter)", binary: "gpython", ext: "py",
	},
	{
		name: "Yaegi", link: "https://github.com/traefik/yaegi",
		typ: "Yaegi (Interpreter)", binary: "yaegi", ext: "yaegi",
		flags: []string{"run"},
	},
	{
		name: "otto", link: "https://github.com/robertkrimen/otto",
		typ: "JavaScript (Interpreter)", binary: "otto", ext: "js",
	},
	{
		name: "Anko", link: "https://github.com/mattn/anko",
		typ: "Anko (Interpreter)", binary: "anko", ext: "ank",
	},
	{name: "-"},
	{
		name:    "Go",
		typ:     "Go (Native)",
		special: specialGo,
	},
	{
		name: "Lua", typ: "Lua (Native)", binary: "lua", ext: "lua",
	},
	{
		name: "Python", typ: "Python (Native)", binary: "python3", ext: "py",
	},
}

func main() {
	n := flag.Int("n", 35, "fibonacci input (detailed mode)")
	count := flag.Int("count", 3, "number of runs (minimum is reported)")
	timeout := flag.Duration("timeout", 60*time.Second, "per-run timeout")
	markdown := flag.Bool("markdown", false, "print comparison table in markdown")
	dir := flag.String("dir", "testdata/bench", "directory containing benchmark scripts")
	flag.Parse()

	if *markdown {
		printMarkdown(*dir, *count, *timeout)
		return
	}

	for _, b := range benchmarks {
		runDetailed(b, *n, *count, *timeout)
	}
}

// --- detailed mode ---

func runDetailed(b benchmark, n, count int, timeout time.Duration) {
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

// --- markdown mode ---

func printMarkdown(dir string, count int, timeout time.Duration) {
	type result struct {
		fib  time.Duration
		fibt time.Duration
		skip bool
	}

	results := make([]result, len(languages))
	for i, lang := range languages {
		if lang.name == "-" {
			continue
		}
		var r result
		switch lang.special {
		case specialTengo:
			fibSrc, err := os.ReadFile(filepath.Join(dir, "fib.tengo"))
			if err != nil {
				fmt.Fprintf(os.Stderr, "warn: %v\n", err)
				r.skip = true
				break
			}
			fibtSrc, err := os.ReadFile(filepath.Join(dir, "fibtc.tengo"))
			if err != nil {
				fmt.Fprintf(os.Stderr, "warn: %v\n", err)
				r.skip = true
				break
			}
			r.fib = measureNativeTengo(fibSrc, count, timeout)
			r.fibt = measureNativeTengo(fibtSrc, count, timeout)
		case specialGo:
			r.fib = measureNativeGo(fib, 35, count)
			r.fibt = measureNativeGo(func(n int) int { return fibTC2(n, 0, 1) }, 35, count)
		default:
			if _, err := exec.LookPath(lang.binary); err != nil {
				r.skip = true
				break
			}
			r.fib = measureExternal(buildCmd(lang, dir, "fib"), count, timeout)
			r.fibt = measureExternal(buildCmd(lang, dir, "fibtc"), count, timeout)
		}
		results[i] = r
	}

	fmt.Println("| | fib(35) | fibt(35) |  Language (Type)  |")
	fmt.Println("| :--- |    ---: |     ---: |  :---: |")
	for i, lang := range languages {
		if lang.name == "-" {
			fmt.Println("| - | - | - | - |")
			continue
		}
		if results[i].skip {
			continue
		}
		nameCol := lang.name
		if lang.link != "" {
			nameCol = fmt.Sprintf("[%s](%s)", lang.name, lang.link)
		}
		fmt.Printf("| %s | `%sms` | `%sms` | %s |\n",
			nameCol, formatMs(results[i].fib), formatMs(results[i].fibt), lang.typ)
	}
}

func buildCmd(lang langEntry, dir, base string) []string {
	script := filepath.Join(dir, base+"."+lang.ext)
	args := make([]string, 0, 2+len(lang.flags))
	args = append(args, lang.binary)
	args = append(args, lang.flags...)
	args = append(args, script)
	return args
}

func measureNativeTengo(src []byte, count int, timeout time.Duration) time.Duration {
	min := time.Duration(math.MaxInt64)
	for i := 0; i < count; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		start := time.Now()
		_, _, _, _, err := runBench(ctx, src)
		d := time.Since(start)
		cancel()
		if err == nil && d < min {
			min = d
		}
	}
	return min
}

func measureNativeGo(fn func(int) int, n, count int) time.Duration {
	min := time.Duration(math.MaxInt64)
	for i := 0; i < count; i++ {
		start := time.Now()
		fn(n)
		if d := time.Since(start); d < min {
			min = d
		}
	}
	return min
}

func measureExternal(args []string, count int, timeout time.Duration) time.Duration {
	min := time.Duration(math.MaxInt64)
	for i := 0; i < count; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		start := time.Now()
		_ = cmd.Run()
		d := time.Since(start)
		cancel()
		if d < min {
			min = d
		}
	}
	return min
}

func formatMs(d time.Duration) string {
	if d == math.MaxInt64 {
		return "?"
	}
	n := (d.Nanoseconds() + 500_000) / 1_000_000
	if n == 0 {
		return "0"
	}
	neg := n < 0
	a := n
	if neg {
		a = -a
	}
	digits := int(math.Log10(float64(a))) + 1
	outLen := digits + (digits-1)/3
	if neg {
		outLen++
	}
	out := make([]rune, outLen)
	var i, j int
	for a > 0 {
		out[outLen-j-1] = '0' + rune(a%10)
		i++
		j++
		if i%3 == 0 && j < outLen {
			out[outLen-j-1] = ','
			j++
		}
		a /= 10
	}
	if neg {
		out[0] = '-'
	}
	return string(out)
}

// --- fibonacci implementations ---

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

// --- low-level bench runner ---

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
