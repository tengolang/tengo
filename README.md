# The Tengo Language

[![GoDoc](https://godoc.org/github.com/tengolang/tengo/v3?status.svg)](https://godoc.org/github.com/tengolang/tengo/v3)
![test](https://github.com/tengolang/tengo/workflows/test/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/tengolang/tengo)](https://goreportcard.com/report/github.com/tengolang/tengo)

**Tengo is a small, dynamic, fast, secure script language for Go.**

This is a fork of [d5/tengo](https://github.com/d5/tengo), kept alive with new
features and bug fixes. Full credit to d5 for the language design and original
implementation.

Tengo is **[fast](#benchmark)** and secure because it's compiled/executed as
bytecode on stack-based VM that's written in native Go.

```golang
/* The Tengo Language */
fmt := import("fmt")

each := func(seq, fn) {
    for x in seq { fn(x) }
}

sum := func(init, seq) {
    each(seq, func(x) { init += x })
    return init
}

fmt.println(sum(0, [1, 2, 3]))   // "6"
fmt.println(sum("", [1, 2, 3]))  // "123"
```

> Test this Tengo code in the
> [Tengo Playground](https://tengolang.com/?s=0c8d5d0d88f2795a7093d7f35ae12c3afa17bea3)

## Features

- Simple and highly readable
  [Syntax](https://github.com/tengolang/tengo/blob/main/docs/tutorial.md)
  - Dynamic typing with type coercion
  - Higher-order functions and closures
  - Immutable values
- [Securely Embeddable](https://github.com/tengolang/tengo/blob/main/docs/interoperability.md)
  and [Extensible](https://github.com/tengolang/tengo/blob/main/docs/objects.md)
- Compiler/runtime written in native Go _(no external deps or cgo)_
- Executable as a
  [standalone](https://github.com/tengolang/tengo/blob/main/docs/tengo-cli.md)
  language / REPL
- Use cases: rules engine, [state machine](https://github.com/d5/go-fsm),
  data pipeline, [transpiler](https://github.com/tengolang/tengo2lua)

## Benchmark

| | fib(35) | score orders | word count | moving avg | filter/map | dispatch |
| :--- | ---: | ---: | ---: | ---: | ---: | ---: |
| [**Tengo**](https://github.com/tengolang/tengo) | `1,171ms` | `76ms` | `30ms` | `62ms` | `61ms` | `115ms` |
| [go-lua](https://github.com/Shopify/go-lua) | `1,619ms` | `149ms` | `1,038ms` | `80ms` | `104ms` | `231ms` |
| [GopherLua](https://github.com/yuin/gopher-lua) | `1,894ms` | `167ms` | `1,051ms` | `72ms` | `153ms` | `268ms` |
| [goja](https://github.com/dop251/goja) | `2,588ms` | `185ms` | `1,195ms` | `235ms` | `163ms` | `364ms` |
| [starlark-go](https://github.com/google/starlark-go) | `4,146ms` | `136ms` | `1,094ms` | `78ms` | `89ms` | `196ms` |
| [gpython](https://github.com/go-python/gpython) | `7,198ms` | `140ms` | `1,160ms` | `110ms` | `76ms` | `214ms` |
| [Yaegi](https://github.com/traefik/yaegi) | `8,853ms` | `182ms` | `944ms` | `38ms` | `79ms` | `328ms` |
| [otto](https://github.com/robertkrimen/otto) | `37,192ms` | `802ms` | `1,547ms` | `811ms` | `521ms` | `1,437ms` |
| [Anko](https://github.com/mattn/anko) | `37,704ms` | `502ms` | `1,235ms` | `220ms` | `139ms` | `625ms` |
| - | - | - | - | - | - | - |
| Go | `56ms` | `27ms` | `30ms` | `25ms` | `26ms` | `29ms` |
| Lua | `588ms` | `38ms` | `1,468ms` | `22ms` | `34ms` | `64ms` |
| Python 2 | `1,223ms` | `57ms` | `23ms` | `39ms` | `47ms` | `89ms` |
| Python 3 | `797ms` | `53ms` | `2,115ms` | `49ms` | `41ms` | `82ms` |

_* [fib(35)](https://github.com/tengolang/tengobench/blob/master/testdata/bench/fib.tengo):
Fibonacci(35)_  
_* [fibt(35)](https://github.com/tengolang/tengobench/blob/master/testdata/bench/fibtc.tengo):
[tail-call](https://en.wikipedia.org/wiki/Tail_call) version of Fibonacci(35)_  
_* **Go** does not read the source code from file, while all other cases do_  
_* See [here](https://github.com/tengolang/tengo/cmd/bench) for commands/codes used_

## Quick Start

```
go get github.com/tengolang/tengo/v3
```

A simple Go example code that compiles/runs Tengo script code with some input/output values:

```golang
package main

import (
	"context"
	"fmt"

	"github.com/tengolang/tengo/v3"
)

func main() {
	// create a new Script instance
	script := tengo.NewScript([]byte(
`each := func(seq, fn) {
    for x in seq { fn(x) }
}

sum := 0
mul := 1
each([a, b, c, d], func(x) {
    sum += x
    mul *= x
})`))

	// set values
	_ = script.Add("a", 1)
	_ = script.Add("b", 9)
	_ = script.Add("c", 8)
	_ = script.Add("d", 4)

	// run the script
	compiled, err := script.RunContext(context.Background())
	if err != nil {
		panic(err)
	}

	// retrieve values
	sum := compiled.Get("sum")
	mul := compiled.Get("mul")
	fmt.Println(sum, mul) // "22 288"
}
```

Or, if you need to evaluate a simple expression, you can use [Eval](https://pkg.go.dev/github.com/tengolang/tengo/v3#Eval) function instead:


```golang
res, err := tengo.Eval(ctx,
	`input ? "success" : "fail"`,
	map[string]interface{}{"input": 1})
if err != nil {
	panic(err)
}
fmt.Println(res) // "success"
```

## References

- [Language Syntax](https://github.com/tengolang/tengo/blob/main/docs/tutorial.md)
- [Object Types](https://github.com/tengolang/tengo/blob/main/docs/objects.md)
- [Runtime Types](https://github.com/tengolang/tengo/blob/main/docs/runtime-types.md)
  and [Operators](https://github.com/tengolang/tengo/blob/main/docs/operators.md)
- [Builtin Functions](https://github.com/tengolang/tengo/blob/main/docs/builtins.md)
- [Interoperability](https://github.com/tengolang/tengo/blob/main/docs/interoperability.md)
- [Tengo CLI](https://github.com/tengolang/tengo/blob/main/docs/tengo-cli.md)
- [Standard Library](https://github.com/tengolang/tengo/blob/main/docs/stdlib.md)
- Syntax Highlighters: [VSCode](https://github.com/lissein/vscode-tengo), [Atom](https://github.com/tengolang/tengo-atom), [Vim](https://github.com/geseq/tengo-vim), [Emacs](https://github.com/CsBigDataHub/tengo-mode)
- **Why the name Tengo?** It's from [1Q84](https://en.wikipedia.org/wiki/1Q84).


