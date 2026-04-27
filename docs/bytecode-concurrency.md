# Bytecode Concurrency Safety

This document explains why compiled Tengo bytecode is safe to share across
multiple concurrent VM instances, what the correct concurrent-use pattern is,
and where the remaining constraints lie.

---

## 1. Key Types

### `Bytecode`
```go
type Bytecode struct {
    FileSet      *parser.SourceFileSet
    MainFunction *CompiledFunction
    Constants    []Object
}
```

### `VM`
```go
type VM struct {
    constants   []Object              // reference into Bytecode.Constants (read-only)
    stack       [StackSize]Object     // per-run
    sp          int                   // per-run
    globals     []Object              // per-run (passed in from Compiled)
    fileSet     *parser.SourceFileSet // reference into Bytecode.FileSet (read-only)
    frames      [MaxFrames]frame      // per-run
    framesIndex int                   // per-run
    curFrame    *frame                // per-run
    curInsts    []byte                // per-run
    ip          int                   // per-run
    aborting    int64                 // atomic
    maxAllocs   int64
    allocs      int64                 // per-run
    err         error                 // per-run
}
```

### `Compiled`
```go
type Compiled struct {
    globalIndexes map[string]int
    bytecode      *Bytecode
    globals       []Object
    maxAllocs     int64
    lock          sync.RWMutex
    fullClone     bool
}
```

---

## 2. Why the VM Never Writes to Shared Bytecode

`NewVM` assigns `bytecode.Constants` and `bytecode.FileSet` by reference; it does **not** copy them.

The VM's run loop accesses `v.constants` at exactly two places, both reads:

- `OpConstant`: `v.stack[v.sp] = v.constants[cidx]`
- `OpClosure`: reads a `*CompiledFunction` template, then constructs a **new**
  `CompiledFunction` on the heap with a fresh `Free` slice. The template in
  `constants` is never modified.

No path in the VM run loop writes back to `v.constants`, `MainFunction.Instructions`,
or `FileSet`. Multiple VM instances therefore share the same `*Bytecode` safely.

---

## 3. The Safe Concurrent-Use Pattern

`Script.Compile()` produces one `*Compiled`. `Compiled.Clone()` produces a
shallow copy that shares the `*Bytecode` pointer but gets its own `globals`
slice (each global value is deep-copied via `Object.Copy()`).

Because `OpSetGlobal` writes to `v.globals`, which points directly at
`c.globals`, concurrent `Run()` calls on the same `*Compiled` would race on
those writes. The exclusive lock in `Run()` serializes them, but that means a
single `*Compiled` effectively runs serially.

**For true parallelism, clone once and run each clone independently:**

```go
compiled, _ := script.Compile()
compiled.Run() // populate globals (define functions, etc.)

var wg sync.WaitGroup
for i := 0; i < N; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        c := compiled.Clone() // each goroutine gets its own globals
        c.Run()
    }()
}
wg.Wait()
```

Clones share the `*Bytecode` safely (read-only during execution) and have
independent `globals` so their `Run()` locks never contend with each other.

---

## 4. Constraints and Caveats

### 4.1 `Bytecode` mutation methods must not be called while VMs are running

Three methods mutate `Bytecode` in place:

- **`ReplaceBuiltinModule`**: replaces elements of `b.Constants` by index.
- **`Decode`**: writes every element of `b.Constants` during deserialisation.
- **`RemoveDuplicates`**: replaces the entire `b.Constants` slice and mutates
  `Instructions` byte slices in-place. The instruction-bytes mutation is
  particularly dangerous because `Instructions` is referenced inside a running
  VM's `curInsts` field.

`Compiled.ReplaceBuiltinModule` is the only caller that acquires the lock
before mutating. The underlying `Bytecode` methods have no synchronisation of
their own and **must only be called before the bytecode is handed to any VM**.
This is documented in their godoc.

### 4.2 `Clone()` shares `*Bytecode`: do not mutate it directly

Calling `c.bytecode.ReplaceBuiltinModule()` directly on a clone (bypassing the
`Compiled` wrapper) mutates the bytecode shared by all clones without any lock.
Always go through `Compiled.ReplaceBuiltinModule`, which handles the
copy-on-write upgrade automatically.

### 4.3 `MaxStringLen` / `MaxBytesLen` are process-wide variables

```go
var (
    MaxStringLen = 2147483647
    MaxBytesLen  = 2147483647
)
```

These are read inside the VM during string and bytes operations. They must not
be changed after the first VM starts executing; doing so is a data race. This
is documented in their godoc.

---

## 5. What Is Safe Without Restrictions

| Pattern | Why safe |
|---------|----------|
| Multiple goroutines each `Clone()` then `Run()` | Independent `globals`; shared bytecode is read-only during execution |
| `Compiled.Get()` / `IsDefined()` concurrent with `Run()` | Use `RLock`; serialised by the exclusive `Lock` in `Run()` |
| `Compiled.Set()` before `Run()` | Uses exclusive `Lock`; writes complete before VM reads |
| `TrueValue`, `FalseValue`, `UndefinedValue` singletons | Never mutated after package init |
| `builtinFuncs` slice | Initialised at startup, never written after |
| `VM.Abort()` | Uses `atomic.StoreInt64` |
| Closure creation via `OpClosure` | Creates a new `*CompiledFunction` per invocation; never mutates the template |

---

## 6. Race-Detector Coverage

`TestCompiledConcurrentRun` in `script_test.go` exercises 100 concurrent
clones under `go test -race`. Run it with:

```
go test -race ./...
```
