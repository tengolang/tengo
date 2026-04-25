# Interoperability  

## Table of Contents

- [Using Scripts](#using-scripts)
  - [Calling a Tengo Function from Go](#calling-a-tengo-function-from-go)
  - [Exposing Go Functions to Tengo](#exposing-go-functions-to-tengo)
  - [Type Conversion Table](#type-conversion-table)
  - [User Types](#user-types)
- [Sandbox Environments](#sandbox-environments)
- [Concurrency](#concurrency)
- [Pausing and Resuming a VM](#pausing-and-resuming-a-vm)
- [Tracing and Hooks](#tracing-and-hooks)
- [Compiler and VM](#compiler-and-vm)

## Using Scripts

Embedding and executing the Tengo code in Go is very easy. At a high level,
this process is like:

- create a [Script](https://pkg.go.dev/github.com/tengolang/tengo/v3#Script) instance with
your code,
- _optionally_ add some
[Script Variables](https://pkg.go.dev/github.com/tengolang/tengo/v3#Variable) to Script,
- compile or directly run the script,
- retrieve _output_ values from the
[Compiled](https://pkg.go.dev/github.com/tengolang/tengo/v3#Compiled) instance.

The following is an example where a Tengo script is compiled and run with no
input/output variables.

```golang
import "github.com/tengolang/tengo/v3"

var code = `
reduce := func(seq, fn) {
    s := 0
    for x in seq { fn(x, s) }
    return s
}

print(reduce([1, 2, 3], func(x, s) { s += x }))
`

func main() {
    s := tengo.NewScript([]byte(code))
    if _, err := s.Run(); err != nil {
        panic(err)
    }
}
```

Here's another example where an input variable is added to the script, and, an
output variable is accessed through
[Variable.Int](https://pkg.go.dev/github.com/tengolang/tengo/v3#Variable.Int) function:

```golang
import (
    "fmt"

    "github.com/tengolang/tengo/v3"
)

func main() {
    s := tengo.NewScript([]byte(`a := b + 20`))

    // define variable 'b'
    _ = s.Add("b", 10)

    // compile the source
    c, err := s.Compile()
    if err != nil {
        panic(err)
    }

    // run the compiled bytecode
    // a compiled bytecode 'c' can be executed multiple times without re-compiling it
    if err := c.Run(); err != nil {
        panic(err)
    }

    // retrieve value of 'a'
    a := c.Get("a")
    fmt.Println(a.Int())           // prints "30"

    // re-run after replacing value of 'b'
    if err := c.Set("b", 20); err != nil {
        panic(err)
    }
    if err := c.Run(); err != nil {
        panic(err)
    }
    fmt.Println(c.Get("a").Int())  // prints "40"
}
```

A variable `b` is defined by the user before compilation using
[Script.Add](https://pkg.go.dev/github.com/tengolang/tengo/v3#Script.Add) function. Then a
compiled bytecode `c` is used to execute the bytecode and get the value of
global variables. In this example, the value of global variable `a` is read
using [Compiled.Get](https://pkg.go.dev/github.com/tengolang/tengo/v3#Compiled.Get)
function. See
[documentation](https://pkg.go.dev/github.com/tengolang/tengo/v3#Variable) for the
full list of variable value functions.

Value of the global variables can be replaced using
[Compiled.Set](https://pkg.go.dev/github.com/tengolang/tengo/v3#Compiled.Set) function.
But it will return an error if you try to set the value of un-defined global
variables _(e.g. trying to set the value of `x` in the example)_.

### Calling a Tengo Function from Go

Tengo doesn't have a direct "call function by name" API, but you can invoke
a named function defined in a script by appending an assignment to a reserved
result variable before compilation.

The key is to reserve both the argument and result variable names with
`Script.Add` _before_ compiling, so the names are guaranteed to exist in the
global scope regardless of what the script defines.

```golang
import (
    "fmt"

    "github.com/tengolang/tengo/v3"
)

const (
    resultVar = "__result"
    arg0Var   = "__arg0"
)

var source = `
greetUser := func(name) {
    return "Hello, " + name
}
`

func main() {
    s := tengo.NewScript([]byte(source + "\n" + resultVar + " := greetUser(" + arg0Var + ")"))
    _ = s.Add(resultVar, nil)
    _ = s.Add(arg0Var, "")

    compiled, err := s.Compile()
    if err != nil {
        panic(err)
    }

    // Clone before each run to avoid sharing state between calls.
    c := compiled.Clone()
    if err := c.Set(arg0Var, "Alice"); err != nil {
        panic(err)
    }
    if err := c.Run(); err != nil {
        panic(err)
    }
    fmt.Println(c.Get(resultVar).String()) // Hello, Alice
}
```

Because the bytecode is compiled once and cloned per call, the cost of
invoking the function repeatedly is just `Clone` + `Set` + `Run` — no
re-compilation.

**Limitations:**
- All arguments must be expressible as pre-defined variables (see
  [Type Conversion Table](#type-conversion-table)).
- The function must be defined at the top level of the script.
- For functions with multiple arguments, add one reserved variable per
  argument slot.

### Exposing Go Functions to Tengo

Any Go function can be made callable from a Tengo script by wrapping it in a
`tengo.UserFunction` and registering it with `Script.Add` before compilation.

```golang
s := tengo.NewScript([]byte(`result := add(3, 4)`))

s.Add("add", &tengo.UserFunction{
    Name: "add",
    Value: func(args ...tengo.Object) (tengo.Object, error) {
        if len(args) != 2 {
            return nil, tengo.ErrWrongNumArguments
        }
        x, ok := args[0].(tengo.Int)
        if !ok {
            return nil, tengo.ErrInvalidArgumentType{
                Name: "first", Expected: "int", Found: args[0].TypeName(),
            }
        }
        y, ok := args[1].(tengo.Int)
        if !ok {
            return nil, tengo.ErrInvalidArgumentType{
                Name: "second", Expected: "int", Found: args[1].TypeName(),
            }
        }
        return tengo.Int{Value: x.Value + y.Value}, nil
    },
})

compiled, err := s.Compile()
// ...
if err := compiled.Run(); err != nil {
    panic(err)
}
fmt.Println(compiled.Get("result").Int()) // 7
```

#### Returning Go values with `FromInterface`

When the Go function works with native Go types (`map[string]interface{}`,
`[]interface{}`, primitives, etc.) use `tengo.FromInterface` to convert them
to `tengo.Object` values rather than building the Tengo types by hand.

```golang
var store = map[string]interface{}{
    "host": "example.com",
    "port": 8080,
    "tags": []interface{}{"prod", "v2"},
}

s.Add("get_config", &tengo.UserFunction{
    Name: "get_config",
    Value: func(args ...tengo.Object) (tengo.Object, error) {
        if len(args) != 1 {
            return nil, tengo.ErrWrongNumArguments
        }
        key, ok := args[0].(*tengo.String)
        if !ok {
            return nil, tengo.ErrInvalidArgumentType{
                Name: "first", Expected: "string", Found: args[0].TypeName(),
            }
        }
        val, found := store[key.Value]
        if !found {
            return tengo.UndefinedValue, nil
        }
        obj, err := tengo.FromInterface(val)
        if err != nil {
            return &tengo.Error{Value: &tengo.String{Value: err.Error()}}, nil
        }
        return obj, nil
    },
})
```

`tengo.ToInterface(obj)` performs the reverse conversion — from a
`tengo.Object` back to a plain Go value — which is useful when passing
Tengo values into existing Go APIs.

#### Error handling conventions

There are two ways to signal failure from a Go function:

1. **Return a Go `error`** — the VM propagates it as a runtime error and
   the script halts (unless the call is wrapped in a Tengo error-check).

2. **Return a `*tengo.Error` object** — the function succeeds at the VM
   level but the script can inspect `result.is_error` or pattern-match on
   the returned value. This mirrors the Tengo idiom of returning `{ok,
   error}` pairs and is preferred when the failure is a domain error rather
   than a programming mistake.

```golang
// Domain error (caller decides how to handle it):
return &tengo.Error{Value: &tengo.String{Value: "key not found"}}, nil

// Programming error (halts the VM):
return nil, tengo.ErrWrongNumArguments
```

#### Returning multiple values

Go functions return a single `tengo.Object`. There are two patterns for
exposing multiple values depending on how the Tengo caller will consume them.

**Return an `*Array`** when the caller will index into the result:

```golang
// In Go:
return &tengo.Array{Value: []tengo.Object{
    tengo.Int{Value: quotient},
    tengo.Int{Value: remainder},
}}, nil
```

```tengo
// In Tengo — index into the result:
res := divmod(17, 5)
q := res[0]
r := res[1]
```

**Return a `*MultiValue`** to support Tengo's multi-assignment destructuring
syntax directly:

```golang
// In Go:
return &tengo.MultiValue{Values: []tengo.Object{
    tengo.Int{Value: quotient},
    tengo.Int{Value: remainder},
}}, nil
```

```tengo
// In Tengo — destructures naturally:
q, r := divmod(17, 5)
```

`*MultiValue` is the same internal type produced by `return a, b` in Tengo
itself, so the two approaches are fully compatible from the caller's
perspective.

### Type Conversion Table

When adding a Variable
_([Script.Add](https://pkg.go.dev/github.com/tengolang/tengo/v3#Script.Add))_, Script
converts Go values into Tengo values based on the following conversion table.

| Go Type | Tengo Type | Note |
| :--- | :--- | :--- |
|`nil`|`Undefined`||
|`string`|`String`||
|`int64`|`Int`||
|`int`|`Int`||
|`bool`|`Bool`||
|`rune`|`Char`||
|`byte`|`Char`||
|`float64`|`Float`||
|`[]byte`|`Bytes`||
|`time.Time`|`Time`||
|`error`|`Error{String}`|use `error.Error()` as String value|
|`map[string]Object`|`Map`||
|`map[string]interface{}`|`Map`|individual elements converted to Tengo objects|
|`[]Object`|`Array`||
|`[]interface{}`|`Array`|individual elements converted to Tengo objects|
|`Object`|`Object`|_(no type conversion performed)_|

### User Types

Users can add and use a custom user type in Tengo code by implementing
[Object](https://pkg.go.dev/github.com/tengolang/tengo/v3#Object) interface. Tengo runtime
will treat the user types in the same way it does to the runtime types with no
performance overhead. See
[Object Types](https://github.com/tengolang/tengo/blob/main/docs/objects.md) for
more details.

## Sandbox Environments

To securely compile and execute _potentially_ unsafe script code, you can use
the following Script functions.

### Script.SetImports(modules *objects.ModuleMap)

SetImports sets the import modules with corresponding names. Script **does not**
include any modules by default. You can use this function to include the
[Standard Library](https://github.com/tengolang/tengo/blob/main/docs/stdlib.md).

```golang
s := tengo.NewScript([]byte(`math := import("math"); a := math.abs(-19.84)`))

s.SetImports(stdlib.GetModuleMap("math"))
// or, to include all stdlib at once
s.SetImports(stdlib.GetModuleMap(stdlib.AllModuleNames()...))
```

You can also include Tengo's written module using `objects.SourceModule`
(which implements `objects.Importable`).

```golang
s := tengo.NewScript([]byte(`double := import("double"); a := double(20)`))

mods := tengo.NewModuleMap()
mods.AddSourceModule("double", []byte(`export func(x) { return x * 2 }`))
s.SetImports(mods)
```

To dynamically load or generate code for imported modules, implement and
provide a `tengo.ModuleGetter`.

```golang
type DynamicModules struct {
  mods tengo.ModuleGetter
  fallback func (name string) tengo.Importable
}
func (dm *DynamicModules) Get(name string) tengo.Importable {
  if mod := dm.mods.Get(name); mod != nil {
    return mod
  }
  return dm.fallback()
}
// ...
mods := &DynamicModules{
  mods: stdlib.GetModuleMap("math"),
  fallback: func(name string) tengo.Importable {
    src := ... // load or generate src for `name`
    return &tengo.SourceModule{Src: src}
  },
}
s := tengo.NewScript(`foo := import("foo")`)
s.SetImports(mods)
```

### Script.SetMaxAllocs(n int64)

SetMaxAllocs sets the maximum number of object allocations. Note this is a
cumulative metric that tracks only the object creations. Set this to a negative
number (e.g. `-1`) if you don't need to limit the number of allocations.

### Script.EnableFileImport(enable bool)

EnableFileImport enables or disables module loading from the local files. It's
disabled by default.

### tengo.MaxStringLen

Sets the maximum byte-length of string values. This limit applies to all
running VM instances in the process. Also it's not recommended to set or update
this value while any VM is executing.

### tengo.MaxBytesLen

Sets the maximum length of bytes values. This limit applies to all running VM
instances in the process. Also it's not recommended to set or update this value
while any VM is executing.

## Concurrency

A compiled script (`Compiled`) can be used to run the code multiple
times by a goroutine. If you want to run the compiled script by multiple
goroutine, you should use `Compiled.Clone` function to make a copy of Compiled
instances.

### Compiled.Clone()

Clone creates a new copy of Compiled instance. Cloned copies are safe for
concurrent use by multiple goroutines. 

```golang
for i := 0; i < concurrency; i++ {
    go func(compiled *tengo.Compiled) {
        // inputs
        _ = compiled.Set("a", rand.Intn(10))
        _ = compiled.Set("b", rand.Intn(10))
        _ = compiled.Set("c", rand.Intn(10))

        if err := compiled.Run(); err != nil {
            panic(err)
        }

        // outputs
        d = compiled.Get("d").Int()
        e = compiled.Get("e").Int()
    }(compiled.Clone()) // Pass the cloned copy of Compiled
}
```

## Pausing and Resuming a VM

A running VM can be suspended at any instruction boundary and resumed later
from the exact same point. This is useful for implementing cooperative
scheduling, debuggers, step-through execution, or any scenario where an
external controller needs to stop and restart a script.

Because the `Script`/`Compiled` API creates a new `VM` internally on each
`Run()` call, pause/resume requires working with `VM` directly. Use
`Compiled.Bytecode()` and `Compiled.Globals()` to obtain the pieces needed
to construct one:

```golang
compiled, err := script.Compile()
// ...

vm := tengo.NewVM(compiled.Bytecode(), compiled.Globals(), -1)
```

### VM.Pause()

`Pause` signals the VM to stop after the current instruction completes.
It is safe to call from any goroutine. The VM state is fully preserved —
the instruction pointer, call stack, and all globals remain intact.

```golang
done := make(chan error, 1)
go func() { done <- vm.Run() }()

// ... some time later, from any goroutine:
vm.Pause()
err := <-done   // Run() returns nil (not an error)
```

### VM.IsPaused()

`IsPaused` reports whether the VM is currently in a paused state.
Use it after `Run()` or `Resume()` returns to distinguish a clean pause
from normal completion.

```golang
if vm.IsPaused() {
    // script is mid-execution; can be resumed
} else {
    // script ran to completion (or was aborted)
}
```

### VM.Resume()

`Resume` clears the pause flag and continues execution from where it
stopped. It must be called from the goroutine that owns the VM — i.e.
after `Run()` or a previous `Resume()` has returned. It returns the same
kind of error as `Run()`.

```golang
if err := vm.Resume(); err != nil {
    // runtime error inside the script
}
```

### Full example

```golang
import (
    "fmt"

    "github.com/tengolang/tengo/v3"
)

func main() {
    s := tengo.NewScript([]byte(`
        count := 0
        for i := 0; i < 1000000; i++ {
            count = count + 1
        }
    `))

    compiled, err := s.Compile()
    if err != nil {
        panic(err)
    }

    vm := tengo.NewVM(compiled.Bytecode(), compiled.Globals(), -1)

    done := make(chan error, 1)
    go func() { done <- vm.Run() }()

    // Pause mid-execution.
    vm.Pause()
    <-done

    if vm.IsPaused() {
        fmt.Println("paused at count =", compiled.Get("count").Int())
    }

    // Resume to completion.
    if err := vm.Resume(); err != nil {
        panic(err)
    }
    fmt.Println("final count =", compiled.Get("count").Int()) // 1000000
}
```

**Notes:**
- `Pause()` is safe from any goroutine; `Resume()` must be called from
  the goroutine that owns the VM.
- If `Pause()` is called after the script has already finished, `IsPaused()`
  returns false and `Resume()` returns immediately.
- `Abort()` permanently stops execution; `Pause()` + `Resume()` allows it
  to continue.

## Tracing and Hooks

The VM supports a hook mechanism similar to Python's `sys.settrace()` and
Lua's `debug.sethook()`. A single Go callback can be registered to fire at
any combination of three events: function calls, function returns, and
source-line changes. This is the foundation for debuggers, code-coverage
collectors, profilers, and execution inspectors.

### VM.SetHook(fn HookFunc, mask HookMask)

`SetHook` installs the hook function `fn` and enables the events selected by
`mask`. Pass `fn=nil` (or `mask=0`) to remove the hook.

```golang
vm.SetHook(func(v *tengo.VM, info tengo.HookInfo) {
    fmt.Printf("event=%v depth=%d pos=%s\n",
        info.Event, info.Depth, info.Pos)
}, tengo.HookMaskCall | tengo.HookMaskReturn | tengo.HookMaskLine)
```

The three mask constants:

| Constant | Event fired |
| :--- | :--- |
| `HookMaskCall` | A compiled function is called. `info.Pos` is the function's definition site. |
| `HookMaskReturn` | A function is about to return. `info.RetVal` carries the return value. |
| `HookMaskLine` | Execution enters a new source line. `info.Pos.Line` is the new line number. |

`HookInfo` fields:

| Field | Type | Description |
| :--- | :--- | :--- |
| `Event` | `HookEvent` | `HookCall`, `HookReturn`, or `HookLine` |
| `Depth` | `int` | Call-stack depth (1 = script body, +1 per nested call) |
| `Pos` | `parser.SourceFilePos` | Source position (filename, line, column) |
| `RetVal` | `Object` | Return value — set only for `HookReturn`, nil otherwise |

### Performance

Hooks are checked with a bitmask test on every instruction. When no hook is
installed (`mask == 0`) the branch is never taken and there is no measurable
overhead. `HookMaskLine` adds one source-map lookup per instruction when
enabled; the other masks add checks only at the relevant opcodes.

### Building a debugger

The hook integrates cleanly with `Pause()` and `Resume()` (see
[Pausing and Resuming a VM](#pausing-and-resuming-a-vm)): call `v.Pause()`
inside the hook to stop the VM at any event.

```golang
vm := tengo.NewVM(compiled.Bytecode(), compiled.Globals(), -1)

// Break on line 10.
vm.SetHook(func(v *tengo.VM, info tengo.HookInfo) {
    if info.Event == tengo.HookLine && info.Pos.Line == 10 {
        v.Pause()
    }
}, tengo.HookMaskLine)

if err := vm.Run(); err != nil {
    panic(err)
}

if vm.IsPaused() {
    fmt.Println("stopped at line 10")
    // inspect globals, then continue:
    vm.Resume()
}
```

### Collecting code coverage

```golang
covered := make(map[int]bool)

vm.SetHook(func(_ *tengo.VM, info tengo.HookInfo) {
    covered[info.Pos.Line] = true
}, tengo.HookMaskLine)

vm.Run()
fmt.Println("lines executed:", covered)
```

## Compiler and VM

Although it's not recommended, you can directly create and run the Tengo
[Compiler](https://pkg.go.dev/github.com/tengolang/tengo/v3#Compiler), and
[VM](https://pkg.go.dev/github.com/tengolang/tengo/v3#VM) for yourself instead of using
Scripts and Script Variables. It's a bit more involved as you have to manage
the symbol tables and global variables between them, but, basically that's what
Script and Script Variable is doing internally.

_TODO: add more information here_
