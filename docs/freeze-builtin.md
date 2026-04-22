# Proposed builtin: `freeze`

## Motivation

`freeze(x)` recursively converts a value into its fully immutable equivalent.
The primary use case is sharing read-only data across concurrent `Clone()`
executions without copying: a frozen value can be placed in globals before
`Compile()` and then shared across all clones at zero cost, because no clone
can mutate it.

Without `freeze`, deep immutability requires constructing `ImmutableMap` and
`ImmutableArray` values by hand, which is impractical for nested structures
built at runtime.

## Semantics

```
freeze(x) -> immutable equivalent of x
```

- `Array` → `ImmutableArray` (elements recursively frozen)
- `Map` → `ImmutableMap` (values recursively frozen)
- `ImmutableArray` → recurse into elements (may contain mutable children)
- `ImmutableMap` → recurse into values (may contain mutable children)
- Primitives (`Int`, `Float`, `String`, `Bool`, `Bytes`, `Char`, `Time`,
  `Undefined`) → returned as-is (no mutable operations in-language)
- `UserFunction`, `CompiledFunction`, `Error` → returned as-is
- A fully frozen input with no mutable children → same pointer returned,
  no allocation

## Cycle and DAG handling

A memo table (`map[uintptr]Object`, keyed on pointer address) must be
maintained across the recursion:

- Before processing a container, record `ptr → result` in the memo table.
- On revisiting the same pointer, return the already-frozen result.

This handles both cycles (map containing itself) and shared subgraphs
(same object reachable via multiple paths) correctly and without extra
allocations.

## Map ordering

`Map` and `ImmutableMap` are both backed by `map[string]Object` in Go.
Iteration order is undefined (Go deliberately randomizes it). `freeze` makes
no ordering guarantees and introduces no ordering. This is consistent with
existing map behaviour.

## Interaction with Clone()

`Compiled.Clone()` calls `g.Copy()` on every global, which shallow-copies
containers. A frozen global is returned as-is by `Copy()` (all immutable
types return `self` from `Copy()`), so it is shared across clones at zero
cost. This is the main performance motivation.

## Implementation notes

- Add `freeze` to `builtins.go` alongside the existing `copy` builtin.
- Private helper: `func freezeObject(o Object, memo map[uintptr]Object) Object`
- Entry point checks `len(args) == 1`, calls `freezeObject(args[0], make(map[uintptr]Object))`.
- The memo map is allocated once per `freeze` call and threaded through
  recursion; it is not retained afterwards.
- No new opcodes or parser changes required.

## Example

```tengo
config := freeze({
    limits: {max_retries: 3, timeout: 30},
    tags:   ["prod", "v2"],
})

// config, config.limits, and config.tags are all immutable.
// Safe to share across concurrent script clones without copying.
```
