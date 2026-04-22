# Bytecode Concurrency Safety Analysis

This document analyses whether compiled Tengo bytecode is safe to reuse across
multiple VM instances running concurrently. It is intended as a verification
target: a second reviewer should be able to check every claim against the
referenced source locations.

Repository root: `/home/mikael/Project/github/tengo`
Commit analysed: `4414573` (branch `feature/switch-case`)

---

## 1. Key Types

### `Bytecode` (`bytecode.go:13-17`)
```go
type Bytecode struct {
    FileSet      *parser.SourceFileSet
    MainFunction *CompiledFunction
    Constants    []Object
}
```

### `VM` (`vm.go:20-35`)
```go
type VM struct {
    constants   []Object              // reference into Bytecode.Constants
    stack       [StackSize]Object     // per-run
    sp          int                   // per-run
    globals     []Object              // per-run (passed in from Compiled)
    fileSet     *parser.SourceFileSet // reference into Bytecode.FileSet
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

### `Compiled` (`script.go:198-205`)
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

## 2. VM Construction and Execution

`NewVM` (`vm.go:38-60`) assigns `bytecode.Constants` and `bytecode.FileSet` by
reference into the VM. It does **not** copy them.

`VM.Run()` (`vm.go:68-95`) resets only per-run fields (`sp`, `framesIndex`,
`ip`, `allocs`). The references to `constants` and `fileSet` remain pointing
at the shared Bytecode.

### Does the VM ever write to `v.constants`?

The VM's main loop accesses `v.constants` at exactly two places:

- `vm.go:106` — `v.stack[v.sp] = v.constants[cidx]` (read, `OpConstant`)
- `vm.go:751` — `fn, ok := v.constants[constIndex].(*CompiledFunction)` (read, `OpClosure`)

**Neither assignment writes back to `v.constants`.** The slice is used purely
as a read-only lookup table during execution.

### OpClosure does not mutate the template

`OpClosure` (`vm.go:747-778`) reads a `*CompiledFunction` from constants as a
template, then constructs a brand-new `CompiledFunction` on the heap:

```go
cl := &CompiledFunction{
    Instructions:  fn.Instructions,  // shared byte-slice, read-only
    NumLocals:     fn.NumLocals,
    NumParameters: fn.NumParameters,
    VarArgs:       fn.VarArgs,
    SourceMap:     fn.SourceMap,     // shared map, read-only
    Free:          free,             // NEW slice, per-closure-instance
}
```

`free` is built from the current VM stack (`vm.go:756-765`); it is unique to
the running VM. The template in `constants` is never modified.

### Conclusion for VM execution

During normal execution a VM **only reads** from shared Bytecode state. No
writes to `constants`, `MainFunction.Instructions`, or `FileSet` occur inside
the VM run loop. Multiple VM instances can therefore share the same `*Bytecode`
safely during execution.

---

## 3. The Intended Safe-Concurrency Pattern

`Script.Compile()` (`script.go:92-143`) produces one `*Compiled` with
`fullClone: true`.

`Compiled.Clone()` (`script.go:260-278`) produces a shallow copy:

```go
clone := &Compiled{
    globalIndexes: c.globalIndexes,          // SHARED (read-only map)
    bytecode:      c.bytecode,               // SHARED *Bytecode pointer
    globals:       make([]Object, len(...)), // NEW slice
    fullClone:     false,
}
// deep-copies global Object values into the new slice
for idx, g := range c.globals {
    if g != nil { clone.globals[idx] = g.Copy() }
}
```

Each clone gets its own `globals` slice, so script-level variables are
isolated. The shared `bytecode` is safe because — as established in §2 — the
VM never writes to it.

**The correct concurrent-use pattern is therefore:**

```go
compiled, _ := script.Compile()  // once

var wg sync.WaitGroup
for i := 0; i < N; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        c := compiled.Clone()   // per goroutine
        c.Run()
    }()
}
wg.Wait()
```

---

## 4. Identified Issues

### 4.1 `Compiled.Run()` exclusive lock is correct but the reason was misunderstood (RESOLVED — analysis correction)

Initial analysis incorrectly labelled the `Lock()` in `Run()` a design flaw.
Race-detector testing proved it is necessary.

The VM passes `c.globals` by reference to `NewVM`. During execution, `OpSetGlobal`
(`vm.go:250`) writes directly into that slice:

```go
v.globals[globalIndex] = v.stack[v.sp]
```

Because `c.globals` is shared across all calls to `Run()` on the same `*Compiled`,
concurrent runs would race on those writes. The exclusive lock correctly serializes
them.

`Run()` and `RunContext()` must remain `Lock()`/`Unlock()`. `Set()` also writes
to `c.globals` and must remain exclusive.

**Implication for users:** Concurrent execution requires `Clone()`, not concurrent
calls to `Run()` on the same `Compiled`. Each clone has its own `globals` slice
so its `Run()` lock contends with nothing.

**Verification task for reviewer:** Confirm `OpSetGlobal` and `OpSetSelGlobal`
(`vm.go:246-280`) write to `v.globals`, and that `v.globals` points directly at
`c.globals` (set at `NewVM`, `vm.go:49`). Confirm `Clone()` allocates a new
`globals` slice (`script.go:267`) so clone runs are truly independent.

### 4.2 `Bytecode` mutation methods must not be called while VMs are running (MEDIUM — documentation gap)

Three methods mutate `Bytecode` in place:

**`ReplaceBuiltinModule`** (`bytecode.go:84-97`) replaces elements of
`b.Constants` by index.

**`Decode`** (`bytecode.go:100-126`) writes every element of `b.Constants`
during deserialisation.

**`RemoveDuplicates`** (`bytecode.go:130-218`) replaces the entire
`b.Constants` slice and mutates `Instructions` byte slices in-place via
`updateConstIndexes` (`bytecode.go:278-305`). The instruction-bytes mutation is
particularly dangerous: `Instructions` is referenced from inside the VM's
`curInsts` field as a plain `[]byte`.

`Compiled.ReplaceBuiltinModule` (`script.go:285-307`) is the only caller that
protects the mutation with a lock. The underlying `Bytecode` methods have no
synchronisation.

**Proposed fix:** Document clearly that `Decode` and `RemoveDuplicates` must
only be called before the bytecode is handed to any VM. `ReplaceBuiltinModule`
on `*Bytecode` directly (not via `*Compiled`) should be documented as unsafe
for concurrent use.

**Verification task for reviewer:** Trace every call site of
`bytecode.RemoveDuplicates()`. Currently there is one: `script.go:127`. Confirm
it is always called before the `*Bytecode` is exposed to a `VM`.

### 4.3 `MaxStringLen` / `MaxBytesLen` are process-wide mutable globals (LOW)

`tengo.go:10-18`:
```go
var (
    MaxStringLen = 2147483647
    MaxBytesLen  = 2147483647
)
```

These variables are read inside `objects.go` during string/bytes operations
inside the VM. If one goroutine changes them while another VM is mid-execution,
the change is visible immediately and without synchronisation.

**Proposed fix (conservative):** Document that these must not be changed after
the first VM starts. Or move them to per-VM or per-Script configuration fields.

**Verification task for reviewer:** Search for all write sites of `MaxStringLen`
and `MaxBytesLen` in the codebase and confirm they are all initialisation-time
assignments, not runtime mutations.

### 4.4 `Compiled.Clone()` shares `bytecode` without flagging its mutability (LOW — documentation gap)

Clones set `fullClone: false`. If `ReplaceBuiltinModule` is then called on a
clone, it first detects `!c.fullClone` and upgrades to a private bytecode copy
(`script.go:289-303`). This is correct.

However nothing prevents calling `c.bytecode.ReplaceBuiltinModule()` directly
(bypassing the `Compiled` wrapper), which would mutate the bytecode shared by
all clones without any lock.

**Proposed fix:** Make `Bytecode` fields unexported, or keep them exported but
add a warning in the `Clone()` godoc.

---

## 5. What is Safe Today (No Changes Needed)

| Pattern | Why it is safe |
|---------|---------------|
| Multiple goroutines each `Clone()` then `Run()` | Each clone has independent `globals`; shared bytecode is read-only during VM execution |
| `Compiled.Get()` / `IsDefined()` concurrent with `Run()` | Both use `RLock`; `Run()` currently uses full `Lock` (so actually serialised today, unnecessarily) |
| `Compiled.Set()` before `Run()` | Uses full `Lock`; writes to globals before VM reads them |
| `TrueValue`, `FalseValue`, `UndefinedValue` singletons | Immutable `*Bool` / `*Undefined`; never mutated |
| `builtinFuncs` slice | Initialised at startup, never written after |
| `VM.Abort()` | Uses `atomic.StoreInt64` |
| Closure creation via `OpClosure` | Creates a new `*CompiledFunction` per invocation; never mutates the template in constants |

---

## 6. Recommended Race-Detector Test

The following test (to be added in `script_test.go` or a new
`concurrent_test.go`) would surface any actual data races via `go test -race`:

```go
func TestCompiledConcurrentRun(t *testing.T) {
    s := tengo.NewScript([]byte(`x := x + 1`))
    s.Add("x", 0)
    compiled, err := s.Compile()
    if err != nil {
        t.Fatal(err)
    }

    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            c := compiled.Clone()
            if err := c.Run(); err != nil {
                t.Error(err)
            }
        }()
    }
    wg.Wait()
}
```

Running `go test -race ./...` should be the first step in any fix verification.

---

## 7. Summary of Proposed Changes

| # | File | Change | Impact |
|---|------|--------|--------|
| 1 | `script.go` | Godoc on `Run()`/`RunContext()`: explain why Lock is necessary and direct users to Clone() for concurrency | Documentation only — Lock stays |
| 2 | `bytecode.go` | Godoc on `Decode`, `RemoveDuplicates`, `ReplaceBuiltinModule`: "must not be called after VM creation" | Documentation only |
| 3 | `script.go:Clone()` | Godoc: warn against calling `bytecode` mutation methods directly on the shared `*Bytecode` | Documentation only |
| 4 | `tengo.go:10-18` | Godoc: "must not be changed after first VM is created; doing so is a data race" | Documentation only |
| 5 | `script_test.go` | Add `TestCompiledConcurrentRun` and `TestCompiledCloneConcurrentRun` with `-race` coverage | Tests — both pass under `go test -race` |
